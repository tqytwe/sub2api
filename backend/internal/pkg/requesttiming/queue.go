package requesttiming

// QueueObservation exposes measured application and final-forward scopes.
// It is derived only from the same persisted trace, never legacy usage timers.
// The forward method includes preparation; it is not upstream inference time.
type QueueObservation struct {
	Status              string `json:"status"`
	Source              string `json:"source"`
	Scope               string `json:"scope"`
	ServerFirstTokenMs  *int64 `json:"server_first_token_ms"`
	ServerDurationMs    int64  `json:"server_duration_ms"`
	ForwardFirstTokenMs *int64 `json:"forward_first_token_ms"`
	ForwardDurationMs   int64  `json:"forward_duration_ms"`
	QueueClamped        bool   `json:"queue_clamped"`
}

func DeriveQueueObservation(s Snapshot) *QueueObservation {
	if s.Truncated || !finite(s.TotalMS) || s.TotalMS <= 0 || s.TotalMS > 9e15 {
		return nil
	}
	var final *Span
	for i := range s.Spans {
		span := &s.Spans[i]
		if span.Name != "forward_attempt" || !finite(span.StartMS) || !finite(span.EndMS) || span.StartMS < 0 || span.EndMS < span.StartMS || span.EndMS > s.TotalMS {
			continue
		}
		if final == nil || span.StartMS > final.StartMS {
			final = span
		}
	}
	if final == nil {
		return nil
	}
	out := &QueueObservation{Status: "measured", Source: "application_request_trace", Scope: "application_entry_and_final_forward_method_including_preparation", ServerDurationMs: int64(s.TotalMS), ForwardDurationMs: int64(final.EndMS - final.StartMS)}
	key := "first_semantic"
	if s.TTFTMode == "visible" {
		key = "first_visible"
	} else if s.TTFTMode != "semantic" {
		return out
	}
	if first, ok := s.Events[key]; ok && finite(first) && first >= final.StartMS && first <= final.EndMS {
		server, forward := int64(first), int64(first-final.StartMS)
		out.ServerFirstTokenMs, out.ForwardFirstTokenMs = &server, &forward
	}
	return out
}
