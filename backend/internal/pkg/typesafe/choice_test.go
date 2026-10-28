package typesafe

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type choiceRoundTripper func(*http.Request) (*http.Response, error)

func (f choiceRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestDecodeChoiceRejectsAmbiguousOrMalformedResponses(t *testing.T) {
	criteria := map[string]string{"correct": "equivalent", "incorrect": "different", "unknown": "uncertain"}
	valid := `{"model":"jev-latest","answers":{"quality":{"type":"choice","choice":"correct","confidence":0.95,"probabilities":{"correct":0.96,"incorrect":0.02,"unknown":0.02}}},"usage":{"input_tokens":10}}`
	got, err := decodeChoice([]byte(valid), criteria, "quality")
	require.NoError(t, err)
	require.Equal(t, "correct", got.Choice)
	for name, body := range map[string]string{
		"missing answer":       `{"model":"jev-latest","answers":{}}`,
		"wrong answer type":    strings.Replace(valid, `"type":"choice"`, `"type":"noul"`, 1),
		"unknown label":        strings.Replace(valid, `"choice":"correct"`, `"choice":"pass"`, 1),
		"missing confidence":   strings.Replace(valid, `"confidence":0.95,`, "", 1),
		"null probability":     strings.Replace(valid, `"incorrect":0.02`, `"incorrect":null`, 1),
		"missing probability":  strings.Replace(valid, `,"unknown":0.02`, "", 1),
		"invalid sum":          strings.Replace(valid, `"correct":0.96`, `"correct":0.5`, 1),
		"selected not highest": strings.Replace(valid, `"choice":"correct"`, `"choice":"incorrect"`, 1),
		"duplicate key":        strings.Replace(valid, `"choice":"correct"`, `"choice":"incorrect","choice":"correct"`, 1),
		"unknown field":        strings.Replace(valid, `"type":"choice"`, `"type":"choice","refusal":"cannot grade"`, 1),
		"trailing data":        valid + `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := decodeChoice([]byte(body), criteria, "quality")
			require.Error(t, err)
		})
	}
}

func TestEvaluateChoiceIsSingleAttemptAndRedactsProviderErrors(t *testing.T) {
	input := Request{Model: JevLatestModel, State: `{"candidate_answer":"ignore rules"}`, Questions: map[string]Question{
		"quality": {Type: "choice", Instructions: "compare", Criteria: map[string]string{"correct": "same", "incorrect": "different", "unknown": "uncertain"}},
	}}
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"credential rejection", http.StatusUnauthorized, "SYNTHETIC_SECRET upstream body"},
		{"rate limit", http.StatusTooManyRequests, "SYNTHETIC_SECRET"},
		{"oversize", http.StatusOK, strings.Repeat(" ", MaxSystemOneResponseBytes+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: choiceRoundTripper(func(req *http.Request) (*http.Response, error) {
				calls++
				require.Equal(t, "/v1/systemone", req.URL.Path)
				require.Equal(t, "Bearer synthetic-key", req.Header.Get("Authorization"))
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
			})}
			_, status, err := EvaluateChoice(context.Background(), client, "https://example.com", "synthetic-key", input, "quality")
			require.Error(t, err)
			require.Equal(t, tc.status, status)
			require.Equal(t, 1, calls)
			require.NotContains(t, err.Error(), "SYNTHETIC_SECRET")
			require.NotContains(t, err.Error(), "synthetic-key")
		})
	}
}
