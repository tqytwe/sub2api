package service

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"

	"github.com/tidwall/gjson"
)

// OpenAI Agents uses the GA Realtime schema and 24 kHz input PCM. Qwen uses
// the earlier event names and 16 kHz input PCM (24 kHz output). Translation is
// activated by the client's session config; native Qwen clients pass through.
type aliyunRealtimeConn struct {
	openAIWSClientConn
	ga       atomic.Bool
	resample bool // only accessed by the downstream-reader goroutine
	pcm      aliyunPCMResampler
}

func (a *aliyunRealtimeConn) WriteJSON(ctx context.Context, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	data, err = a.clientEvent(data)
	if err != nil {
		return err
	}
	return a.openAIWSClientConn.WriteJSON(ctx, json.RawMessage(data))
}

func (a *aliyunRealtimeConn) ReadMessage(ctx context.Context) ([]byte, error) {
	data, err := a.openAIWSClientConn.ReadMessage(ctx)
	if err != nil {
		return nil, err
	}
	return a.serverEvent(data)
}

func (a *aliyunRealtimeConn) clientEvent(data []byte) ([]byte, error) {
	var event map[string]any
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, errors.New("invalid realtime JSON event")
	}
	typ, _ := event["type"].(string)
	if typ == "session.update" {
		session, _ := event["session"].(map[string]any)
		if session != nil {
			if modalities, ok := session["output_modalities"]; ok {
				a.ga.Store(true)
				session["modalities"] = modalities
				// Qwen requires text alongside audio output.
				if modalities := gjson.GetBytes(data, "session.output_modalities").Array(); len(modalities) == 1 && modalities[0].String() == "audio" {
					session["modalities"] = []string{"text", "audio"}
				}
				delete(session, "output_modalities")
			}
			if session["type"] == "realtime" {
				a.ga.Store(true)
				a.resample = true
				delete(session, "type")
			}
			if audio, ok := session["audio"].(map[string]any); ok {
				a.ga.Store(true)
				if input, ok := audio["input"].(map[string]any); ok {
					if format, ok := input["format"].(map[string]any); ok {
						if format["type"] != "audio/pcm" {
							return nil, errors.New("qwen realtime supports PCM audio input only")
						}
						rate, _ := format["rate"].(float64)
						if rate != 0 && rate != 16000 && rate != 24000 {
							return nil, errors.New("qwen realtime input must be 16000 or 24000 Hz PCM")
						}
						a.resample = rate != 16000
						session["input_audio_format"] = "pcm"
					}
					if v, ok := input["turn_detection"]; ok {
						session["turn_detection"] = v
					}
					if v, ok := input["transcription"]; ok {
						session["input_audio_transcription"] = v
					}
				}
				if output, ok := audio["output"].(map[string]any); ok {
					if format, ok := output["format"].(map[string]any); ok && format["type"] != "audio/pcm" {
						return nil, errors.New("qwen realtime supports PCM audio output only")
					}
					session["output_audio_format"] = "pcm"
					if v, ok := output["voice"]; ok {
						session["voice"] = v
					}
					if v, ok := output["speed"]; ok {
						session["speech_rate"] = v
					}
				}
				delete(session, "audio")
			}
			if session["input_audio_format"] == "pcm16" {
				session["input_audio_format"] = "pcm"
				a.resample = true
			}
			if session["output_audio_format"] == "pcm16" {
				session["output_audio_format"] = "pcm"
			}
			if transcript, ok := session["input_audio_transcription"].(map[string]any); ok {
				delete(transcript, "model")
			}
			if turn, ok := session["turn_detection"].(map[string]any); ok {
				if turn["type"] == "semantic_vad" {
					turn["type"] = "smart_turn_v2"
				}
				for _, k := range []string{"eagerness", "interrupt_response", "create_response", "prefix_padding_ms", "idle_timeout_ms"} {
					delete(turn, k)
				}
			}
			if voice, ok := session["voice"].(string); ok && isOpenAIAudioVoice(voice) {
				session["voice"] = "longanqian_v3.1"
			}
			if tools, ok := session["tools"].([]any); ok {
				for i, v := range tools {
					if tool, ok := v.(map[string]any); ok && tool["type"] == "function" && tool["function"] == nil {
						function := map[string]any{}
						for _, k := range []string{"name", "description", "parameters"} {
							if v, ok := tool[k]; ok {
								function[k] = v
							}
						}
						tools[i] = map[string]any{"type": "function", "function": function}
					}
				}
			}
			// GA-only settings have no Qwen equivalent.
			for _, k := range []string{"tracing", "include", "max_output_tokens", "truncation", "parallel_tool_calls", "reasoning", "model"} {
				delete(session, k)
			}
		}
	}
	if typ == "input_audio_buffer.append" && a.resample {
		encoded, _ := event["audio"].(string)
		pcm, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, errors.New("invalid realtime audio base64")
		}
		pcm = a.pcm.convert(pcm)
		event["audio"] = base64.StdEncoding.EncodeToString(pcm)
	}
	if typ == "input_audio_buffer.clear" || typ == "input_audio_buffer.commit" {
		a.pcm = aliyunPCMResampler{}
	}
	if typ == "response.create" {
		if response, ok := event["response"].(map[string]any); ok {
			if modalities, ok := response["output_modalities"]; ok {
				response["modalities"] = modalities
				delete(response, "output_modalities")
			}
			if voice, ok := response["voice"].(string); ok && isOpenAIAudioVoice(voice) {
				response["voice"] = "longanqian_v3.1"
			}
		}
	}
	return json.Marshal(event)
}

func isOpenAIAudioVoice(voice string) bool {
	switch voice {
	case "alloy", "ash", "ballad", "coral", "echo", "sage", "shimmer", "verse", "marin", "cedar", "onyx", "nova", "fable":
		return true
	}
	return false
}

func (a *aliyunRealtimeConn) serverEvent(data []byte) ([]byte, error) {
	// The first greeting precedes session.update. Add the GA discriminator
	// while retaining every native field, so both clients can parse it.
	if !a.ga.Load() && gjson.GetBytes(data, "type").String() != "session.created" {
		return data, nil
	}
	var event map[string]any
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, errors.New("invalid Qwen realtime event")
	}
	typ, _ := event["type"].(string)
	if strings.HasPrefix(typ, "response.audio") {
		event["type"] = strings.Replace(typ, "response.audio", "response.output_audio", 1)
	}
	if typ == "response.text.delta" || typ == "response.text.done" {
		event["type"] = strings.Replace(typ, "response.text", "response.output_text", 1)
	}
	if session, ok := event["session"].(map[string]any); ok {
		session["type"] = "realtime"
		session["output_modalities"] = session["modalities"]
		turn := session["turn_detection"]
		if native, ok := turn.(map[string]any); ok && (native["type"] == "smart_turn" || native["type"] == "smart_turn_v2") {
			turn = map[string]any{"type": "semantic_vad"}
		}
		session["audio"] = map[string]any{
			"input":  map[string]any{"format": map[string]any{"type": "audio/pcm", "rate": 24000}, "turn_detection": turn, "transcription": session["input_audio_transcription"]},
			"output": map[string]any{"format": map[string]any{"type": "audio/pcm", "rate": 24000}, "voice": session["voice"]},
		}
	}
	normalizeAliyunAssistantItem(event["item"])
	if response, ok := event["response"].(map[string]any); ok {
		if output, ok := response["output"].([]any); ok {
			for _, item := range output {
				normalizeAliyunAssistantItem(item)
			}
		}
	}
	return json.Marshal(event)
}

func normalizeAliyunAssistantItem(value any) {
	item, ok := value.(map[string]any)
	if !ok || item["role"] != "assistant" || item["type"] != "message" {
		return
	}
	content, _ := item["content"].([]any)
	for _, value := range content {
		if part, ok := value.(map[string]any); ok {
			switch part["type"] {
			case "text":
				part["type"] = "output_text"
			case "audio":
				part["type"] = "output_audio"
			}
		}
	}
}

// Streaming linear resampling from 24 to 16 kHz. Preserve the fractional
// position and incomplete samples across arbitrary WebSocket frame boundaries.
type aliyunPCMResampler struct {
	pending  []byte
	position int // half-sample units: output positions are 0, 1.5, 3, ...
}

func (r *aliyunPCMResampler) convert(input []byte) []byte {
	r.pending = append(r.pending, input...)
	count := len(r.pending) / 2
	out := make([]byte, 0, len(input)*2/3+4)
	for r.position/2 < count {
		i := r.position / 2
		if r.position%2 != 0 && i+1 >= count {
			break
		}
		v := int32(int16(binary.LittleEndian.Uint16(r.pending[i*2:])))
		if r.position%2 != 0 {
			v = (v + int32(int16(binary.LittleEndian.Uint16(r.pending[(i+1)*2:])))) / 2
		}
		out = binary.LittleEndian.AppendUint16(out, uint16(int16(v)))
		r.position += 3
	}
	consumed := min(r.position/2, count)
	r.pending = append(r.pending[:0], r.pending[consumed*2:]...)
	r.position -= consumed * 2
	return out
}
