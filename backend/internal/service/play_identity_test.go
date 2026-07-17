package service

import "testing"

func TestPublicPlayDisplayNameFallbacks(t *testing.T) {
	tests := []struct {
		name     string
		username string
		email    string
		userID   int64
		want     string
	}{
		{name: "custom username", username: "alice", email: "alice@example.com", userID: 7, want: "alice"},
		{name: "blank username masks email", username: " ", email: "alice@example.com", userID: 7, want: "al***@example.com"},
		{name: "default user masks email", username: "USER", email: "bob@example.com", userID: 9, want: "bo***@example.com"},
		{name: "missing email falls back to id", username: "user", email: "", userID: 11, want: "user-11"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PublicPlayDisplayName(tt.username, tt.email, tt.userID); got != tt.want {
				t.Fatalf("PublicPlayDisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAdminPlayDisplayNameKeepsEmail(t *testing.T) {
	if got := AdminPlayDisplayName("USER", "owner@example.com", 7); got != "owner@example.com" {
		t.Fatalf("AdminPlayDisplayName() = %q, want owner@example.com", got)
	}
}
