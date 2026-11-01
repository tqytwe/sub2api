package service

import (
	"context"
	"errors"
	"testing"
)

type modelBillingCASRepo struct {
	value    string
	revision string
}

func (r *modelBillingCASRepo) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}
func (r *modelBillingCASRepo) GetValue(context.Context, string) (string, error) {
	return r.value, nil
}
func (r *modelBillingCASRepo) Set(context.Context, string, string) error { return nil }
func (r *modelBillingCASRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}
func (r *modelBillingCASRepo) SetMultiple(context.Context, map[string]string) error { return nil }
func (r *modelBillingCASRepo) GetAll(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}
func (r *modelBillingCASRepo) Delete(context.Context, string) error { return nil }

func (r *modelBillingCASRepo) GetValueWithRevision(context.Context, string) (string, string, error) {
	if r.revision == "" {
		return "", "", ErrSettingNotFound
	}
	return r.value, r.revision, nil
}

func (r *modelBillingCASRepo) CompareAndSet(_ context.Context, _, expectedRevision, expectedValue, value string) (string, error) {
	if r.revision != expectedRevision || r.value != expectedValue {
		return "", ErrModelBillingRevisionConflict
	}
	r.value = value
	r.revision = r.revision + "-next"
	return r.revision, nil
}

type modelBillingGroupReader struct{ groups map[int64]*Group }

func (r modelBillingGroupReader) GetByIDLite(_ context.Context, id int64) (*Group, error) {
	g, ok := r.groups[id]
	if !ok {
		return nil, ErrGroupNotFound
	}
	return g, nil
}

func TestModelBillingSettingsRejectInactiveScopedGroup(t *testing.T) {
	repo := &modelBillingCASRepo{}
	svc := NewSettingService(repo, nil)
	svc.SetModelBillingGroupReader(modelBillingGroupReader{groups: map[int64]*Group{
		7: {ID: 7, Status: StatusDisabled},
	}})
	err := svc.ValidateModelBillingConfigForAdmin(context.Background(), ModelBillingConfig{
		Enabled: true,
		Rules:   []ModelBillingRule{{Model: "gpt-5", Multiplier: 2, GroupIDs: []int64{7}}},
	})
	if err == nil {
		t.Fatal("expected inactive group to be rejected")
	}
}

func TestModelBillingSettingsSaveUsesRevisionCAS(t *testing.T) {
	repo := &modelBillingCASRepo{value: `{"enabled":false,"rules":[]}`, revision: "r1"}
	svc := NewSettingService(repo, nil)
	cfg, err := svc.GetModelBillingConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Revision != "r1" {
		t.Fatalf("revision = %q, want r1", cfg.Revision)
	}
	saved, err := svc.SaveModelBillingConfig(context.Background(), ModelBillingConfig{Enabled: true, Rules: []ModelBillingRule{{Model: "gpt-5", Multiplier: 2}}}, cfg.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision != "r1-next" {
		t.Fatalf("saved revision = %q", saved.Revision)
	}
	_, err = svc.SaveModelBillingConfig(context.Background(), ModelBillingConfig{Enabled: true, Rules: []ModelBillingRule{{Model: "gpt-5", Multiplier: 3}}}, "r1")
	if !errors.Is(err, ErrModelBillingRevisionConflict) {
		t.Fatalf("stale save error = %v, want revision conflict", err)
	}
}
