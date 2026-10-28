package service

import (
	"testing"
	"time"
)

func TestChannelOperationsMutationBoundaries(t *testing.T) {
	revision := int64(0)
	tests := []struct {
		name  string
		m     ChannelOperationsMutation
		valid bool
	}{
		{name: "revision required", m: ChannelOperationsMutation{Action: "create_upstream", Name: "Provider"}},
		{name: "blank name", m: ChannelOperationsMutation{Action: "create_upstream", Name: " \t ", Revision: &revision}},
		{name: "control name", m: ChannelOperationsMutation{Action: "create_upstream", Name: "bad\nname", Revision: &revision}},
		{name: "create", m: ChannelOperationsMutation{Action: "create_upstream", Name: " Provider ", Revision: &revision}, valid: true},
		{name: "rename requires id", m: ChannelOperationsMutation{Action: "update_upstream", Name: "Provider", Revision: &revision}},
		{name: "delete requires confirmation", m: ChannelOperationsMutation{Action: "delete_upstream", UpstreamID: 1, Revision: &revision}},
		{name: "delete", m: ChannelOperationsMutation{Action: "delete_upstream", UpstreamID: 1, Revision: &revision, Confirm: true}, valid: true},
		{name: "initial assignment", m: ChannelOperationsMutation{Action: "assign", AccountID: 1, UpstreamID: 2, Revision: &revision}, valid: true},
		{name: "reassignment requires confirmation", m: ChannelOperationsMutation{Action: "assign", AccountID: 1, UpstreamID: 2, ExpectedUpstreamID: 3, Revision: &revision}},
		{name: "reassignment", m: ChannelOperationsMutation{Action: "assign", AccountID: 1, UpstreamID: 2, ExpectedUpstreamID: 3, Revision: &revision, Confirm: true}, valid: true},
		{name: "detach requires previous id", m: ChannelOperationsMutation{Action: "detach", AccountID: 1, Revision: &revision, Confirm: true}},
		{name: "detach requires confirmation", m: ChannelOperationsMutation{Action: "detach", AccountID: 1, ExpectedUpstreamID: 2, Revision: &revision}},
		{name: "detach", m: ChannelOperationsMutation{Action: "detach", AccountID: 1, ExpectedUpstreamID: 2, Revision: &revision, Confirm: true}, valid: true},
		{name: "unknown action", m: ChannelOperationsMutation{Action: "probe", Revision: &revision}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateChannelOperationsMutation(&tt.m)
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v err=%v", tt.valid, err)
			}
		})
	}
}

func TestChannelOperationsFilterBoundaries(t *testing.T) {
	base := ChannelOperationsFilter{From: time.Now().Add(-time.Hour), Until: time.Now(), ModelScope: "public", Dimension: "channel", Page: 1, PageSize: 20}
	if err := ValidateChannelOperationsFilter(base); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*ChannelOperationsFilter){
		func(f *ChannelOperationsFilter) { f.Until = f.From },
		func(f *ChannelOperationsFilter) { f.From = f.Until.Add(-32 * 24 * time.Hour) },
		func(f *ChannelOperationsFilter) { f.ModelScope = "routing" },
		func(f *ChannelOperationsFilter) { f.Dimension = "endpoint" },
		func(f *ChannelOperationsFilter) { f.UpstreamID = 7 },
		func(f *ChannelOperationsFilter) { f.PageSize = 101 },
	} {
		filter := base
		mutate(&filter)
		if ValidateChannelOperationsFilter(filter) == nil {
			t.Fatalf("accepted invalid filter: %+v", filter)
		}
	}
}

func TestChannelOperationsCatalogQueryBoundaries(t *testing.T) {
	valid := ChannelOperationsQuery{Page: 1, PageSize: 20, UpstreamPage: 1, Membership: "all"}
	if err := ValidateChannelOperationsQuery(valid); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*ChannelOperationsQuery){
		func(q *ChannelOperationsQuery) { q.Page = 0 },
		func(q *ChannelOperationsQuery) { q.PageSize = 101 },
		func(q *ChannelOperationsQuery) { q.Membership = "unknown" },
		func(q *ChannelOperationsQuery) { q.UpstreamID = -1 },
	} {
		q := valid
		mutate(&q)
		if ValidateChannelOperationsQuery(q) == nil {
			t.Fatalf("accepted invalid query: %+v", q)
		}
	}
}
