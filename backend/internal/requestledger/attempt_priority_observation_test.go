package requestledger

import (
	"strings"
	"testing"
	"time"
)

func TestObservePrioritySemanticIsBoundedAndIdempotent(t *testing.T) {
	a := &Attempt{startedAt: time.Now().Add(-150 * time.Millisecond)}
	group := int64(42)
	a.ObservePrioritySemantic(" model ", " /v1/responses ", &group)
	a.ObservePrioritySemantic("other", "/other", nil)
	if a.priorityModel != "model" || a.priorityEndpoint != "/v1/responses" || a.priorityGroupID == nil || *a.priorityGroupID != group {
		t.Fatalf("unexpected observation identity: model=%q endpoint=%q group=%v", a.priorityModel, a.priorityEndpoint, a.priorityGroupID)
	}
	if a.priorityFirstSemantic <= 0 {
		t.Fatalf("expected positive first semantic time")
	}
}

func TestObservePrioritySemanticRejectsOversizedDimensions(t *testing.T) {
	a := &Attempt{startedAt: time.Now()}
	a.ObservePrioritySemantic(strings.Repeat("m", 201), "/v1/responses", nil)
	a.ObservePrioritySemantic("model", strings.Repeat("/", 193), nil)
	if a.priorityFirstSemantic != 0 || a.priorityModel != "" {
		t.Fatalf("oversized dimensions must remain unknown: %+v", a)
	}
}
