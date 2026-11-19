package service

import (
	"os"
	"strings"
	"time"
)

// The Fork uses its explicitly deployed worker. Saving administrator settings
// never downloads an executable, starts another service or reroutes passwords.
type OpenAIOAuthReauthWorkerStatus struct {
	Mode   string `json:"mode"`
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

func (s *OpenAIOAuthReauthService) WorkerStatus() OpenAIOAuthReauthWorkerStatus {
	status := OpenAIOAuthReauthWorkerStatus{Mode: "external", State: "unavailable", Reason: "external_not_configured"}
	if s == nil || len(strings.TrimSpace(os.Getenv("OPENAI_REAUTH_WORKER_TOKEN"))) < 32 {
		return status
	}
	status.Reason = "external_offline"
	last := s.workerLastSeen.Load()
	if last != 0 && time.Since(time.Unix(0, last)) <= 26*time.Minute {
		status.State = "running"
		status.Reason = ""
	}
	return status
}

func (s *OpenAIOAuthReauthService) MarkWorkerSeen() {
	if s != nil {
		s.workerLastSeen.Store(time.Now().UnixNano())
	}
}

// Kept as the shared lifecycle boundary used by upstream TOTP tasks; the
// external worker owns process startup and persistent recovery storage.
func (s *OpenAIOAuthReauthService) EnsureWorker() {}

func (s *OpenAIOAuthReauthService) checkWorkerMode(mode string) error { return nil }
