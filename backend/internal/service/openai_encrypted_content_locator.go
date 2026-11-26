package service

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
)

var (
	openAIEncryptedLocator    = regexp.MustCompile(`^The encrypted content ([A-Za-z0-9_-]{4})\.\.\.([A-Za-z0-9_-]{4}) could not be verified\. Reason: Encrypted content could not be decrypted or parsed\.$`)
	openAIEncryptedRelayTrace = regexp.MustCompile(`^（traceid: [0-9a-fA-F]{32}）$`)
)

func isOpenAIInvalidEncryptedContentError(code, message string) bool {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "invalid_encrypted_content":
		return true
	case "thinking_signature_invalid":
		return strings.Contains(message, "The encrypted content") &&
			strings.Contains(message, "could not be verified") &&
			strings.Contains(message, "could not be decrypted or parsed")
	default:
		return false
	}
}

func openAIEncryptedRejectionFields(rejection []byte) (string, string) {
	if len(rejection) > 65536 || !openAIEncryptedJSONUnambiguous(rejection) {
		return "", ""
	}
	root := gjson.ParseBytes(rejection)
	if root.Get("error").IsObject() && root.Get("response.error").IsObject() {
		return "", ""
	}
	if (root.Get("error").IsObject() || root.Get("response.error").IsObject()) && (root.Get("code").Exists() || root.Get("message").Exists()) {
		return "", ""
	}
	if root.Get("response.error").IsObject() {
		root = root.Get("response.error")
	} else if root.Get("error").IsObject() {
		root = root.Get("error")
	}
	if root.Get("code").Type == gjson.Null && root.Get("type").String() == "invalid_request_error" {
		message := root.Get("message")
		if message.Type != gjson.String || !strings.HasPrefix(message.String(), "{") ||
			(root.Get("param").Exists() && root.Get("param").Type != gjson.Null) {
			return "", ""
		}
		decoder := json.NewDecoder(strings.NewReader(message.String()))
		var inner json.RawMessage
		if err := decoder.Decode(&inner); err != nil || !openAIEncryptedJSONUnambiguous(inner) {
			return "", ""
		}
		suffix := strings.TrimSpace(message.String()[decoder.InputOffset():])
		if suffix != "" && !openAIEncryptedRelayTrace.MatchString(suffix) {
			return "", ""
		}
		envelope := gjson.ParseBytes(inner)
		if !envelope.Get("error").IsObject() || envelope.Get("response").Exists() ||
			envelope.Get("code").Exists() || envelope.Get("message").Exists() {
			return "", ""
		}
		root = envelope.Get("error")
	}
	if root.Get("code").Type != gjson.String || (root.Get("message").Exists() && root.Get("message").Type != gjson.String) {
		return "", ""
	}
	return root.Get("code").String(), root.Get("message").String()
}

func openAIRejectedEncryptedDigests(body, rejection []byte) []string {
	code, message := openAIEncryptedRejectionFields(rejection)
	if !isOpenAIInvalidEncryptedContentError(code, message) || !openAIEncryptedJSONUnambiguous(body) {
		return nil
	}
	match := openAIEncryptedLocator.FindStringSubmatch(message)
	genericDiagnostic := message == "The encrypted content could not be verified" ||
		message == "The encrypted content could not be verified." ||
		message == "The encrypted content could not be verified. Reason: Encrypted content could not be decrypted or parsed."
	if len(match) == 0 && !genericDiagnostic && (strings.Contains(message, "...") || strings.ContainsAny(message, "…\r\n") ||
		strings.Contains(strings.ToLower(message), "the encrypted content") || strings.EqualFold(strings.TrimSpace(code), "thinking_signature_invalid")) {
		return nil
	}
	input := gjson.GetBytes(body, "input")
	var candidates []string
	unsafe := false
	ids := map[string]bool{}
	visit := func(item gjson.Result) {
		if !item.IsObject() {
			unsafe = true
			return
		}
		kind := strings.TrimSpace(item.Get("type").String())
		if kind == "agent_message" {
			unsafe = true
		}
		if id := item.Get("id").String(); id != "" {
			if ids[id] {
				unsafe = true
			}
			ids[id] = true
		}
		encrypted := item.Get("encrypted_content")
		if !encrypted.Exists() || encrypted.Type == gjson.Null {
			return
		}
		if encrypted.Type != gjson.String || encrypted.String() == "" || !openAIEncryptedLineageItemType(kind) {
			unsafe = true
			return
		}
		candidates = append(candidates, encrypted.String())
	}
	if input.IsArray() {
		input.ForEach(func(_, item gjson.Result) bool { visit(item); return true })
	} else if input.IsObject() {
		visit(input)
	}
	if unsafe {
		return nil
	}
	if len(match) == 0 {
		if len(candidates) == 1 {
			return []string{openAIEncryptedContentDigest(candidates[0])}
		}
		return nil
	}
	found := ""
	count := 0
	for _, candidate := range candidates {
		if len(candidate) > 8 && strings.HasPrefix(candidate, match[1]) && strings.HasSuffix(candidate, match[2]) {
			count++
			found = candidate
		}
	}
	if count != 1 {
		return nil
	}
	return []string{openAIEncryptedContentDigest(found)}
}

func openAIEncryptedDigestSet(digests []string) map[string]struct{} {
	invalid := make(map[string]struct{}, len(digests))
	for _, digest := range digests {
		invalid[digest] = struct{}{}
	}
	return invalid
}
