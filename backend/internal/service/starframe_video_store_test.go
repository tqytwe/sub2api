//go:build unit

package service

import (
	"context"
	"fmt"
	"sync"
)

type starframeMemoryStore struct {
	mu     sync.Mutex
	tasks  map[string]StarframeVideoTask
	claims map[string]bool
}

func (s *starframeMemoryStore) Claim(_ context.Context, t *StarframeVideoTask) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.claims == nil {
		s.claims = map[string]bool{}
		s.tasks = map[string]StarframeVideoTask{}
	}
	key := fmt.Sprintf("%v:%s", t.Owner, t.ClientTaskID)
	if s.claims[key] {
		return false, nil
	}
	s.claims[key] = true
	s.tasks[t.LocalID] = *t
	return true, nil
}
func (s *starframeMemoryStore) Complete(_ context.Context, t *StarframeVideoTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[t.LocalID] = *t
	return nil
}
func (s *starframeMemoryStore) Get(_ context.Context, id string, o StarframeVideoOwner) (*StarframeVideoTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok || t.Owner != o {
		return nil, fmt.Errorf("not found")
	}
	return &t, nil
}
