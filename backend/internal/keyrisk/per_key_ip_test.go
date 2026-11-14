package keyrisk

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestCrossIPMailTargetsOnlyTheKeysOwner(t *testing.T) {
	s, m := fixture(t)
	cfg := DefaultConfig()
	cfg.AutoEmail = true
	if err := s.Save(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	run(t, s)
	if len(m.to) != 1 || m.to[0] != "one@example.invalid" {
		t.Fatalf("mail attribution: %v", m.to)
	}
	events, err := s.Events(context.Background(), 0)
	if err != nil || len(events) != 1 || events[0].Rule != "key_ip_spread" || events[0].Subject != "11" {
		t.Fatal("wrong event", events, err)
	}
}

func TestExplicitlyAllowedMultiExitKeysDoNotAlert(t *testing.T) {
	for _, tt := range []struct {
		name, allow, deny string
		want              int
	}{
		{"two approved exits", `["192.0.2.1","192.0.2.2"]`, `[]`, 0},
		{"approved CIDR", `["192.0.2.0/24"]`, `[]`, 0},
		{"one unexpected exit", `["192.0.2.1"]`, `[]`, 1},
		{"blacklist takes precedence", `["192.0.2.0/24"]`, `["192.0.2.2"]`, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, m := fixture(t)
			exec(t, s, "UPDATE api_keys SET ip_whitelist=$1,ip_blacklist=$2 WHERE id=11", tt.allow, tt.deny)
			cfg := DefaultConfig()
			cfg.AutoEmail = true
			if err := s.Save(context.Background(), cfg); err != nil {
				t.Fatal(err)
			}
			run(t, s)
			events, err := s.Events(context.Background(), 0)
			if err != nil || len(events) != tt.want || len(m.to) != tt.want {
				t.Fatalf("events=%v mail=%v err=%v", events, m.to, err)
			}
		})
	}
}

func TestRestrictionRequiresCurrentLiveOwnedKey(t *testing.T) {
	s, _ := fixture(t)
	ctx := context.Background()
	run(t, s)
	events, err := s.Events(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	ev := events[0]
	key, err := s.RestrictIPWhitelist(ctx, ev.ID, ev.Generation, []string{"192.0.2.0/24"})
	if err != nil || key != "sk-keyrisk-fixture" {
		t.Fatalf("bad restriction %q %v", key, err)
	}
	if _, err = s.RestrictIPWhitelist(ctx, ev.ID, ev.Generation+1, []string{"198.51.100.0/24"}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("stale generation accepted", err)
	}
	for _, update := range []string{
		"UPDATE api_keys SET status='disabled' WHERE id=11",
		"UPDATE api_keys SET status='active',expires_at=now()-interval '1 second' WHERE id=11",
		"UPDATE api_keys SET expires_at=NULL,deleted_at=now() WHERE id=11",
		"UPDATE api_keys SET deleted_at=NULL,user_id=2 WHERE id=11",
	} {
		exec(t, s, update)
		if _, err = s.RestrictIPWhitelist(ctx, ev.ID, ev.Generation, []string{"198.51.100.0/24"}); !errors.Is(err, sql.ErrNoRows) {
			t.Fatal("invalid target accepted", update, err)
		}
	}
	exec(t, s, "UPDATE api_keys SET user_id=1 WHERE id=11")
	if err = s.SetState(ctx, ev.ID, 99, "resolved"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RestrictIPWhitelist(ctx, ev.ID, ev.Generation, []string{"198.51.100.0/24"}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("resolved episode accepted", err)
	}
}

func TestRetiredRulesCannotNotifyOrRestrictBeforeMigration(t *testing.T) {
	s, m := fixture(t)
	ctx := context.Background()
	exec(t, s, `INSERT INTO key_risk_events(rule,subject,evidence,first_seen,last_seen) VALUES
 ('cross_user_ip','192.0.2.1','{"user_ids":[1],"key_ids":[11]}',now(),now()),
 ('key_ip_spread_linked','11','{"user_ids":[1],"key_ids":[11]}',now(),now());
 INSERT INTO key_risk_recipients(event_id,generation,user_id,key_ids) SELECT id,generation,1,ARRAY[11]::bigint[] FROM key_risk_events;
 INSERT INTO key_risk_outbox(user_id,kind,bucket,manual) VALUES(1,'risk',1,true);
 INSERT INTO key_risk_outbox_items(outbox_id,event_id,generation,key_id,evidence) SELECT 1,id,generation,11,evidence FROM key_risk_events`)
	events, err := s.Events(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range events {
		if _, err = s.NotifyUsers(ctx, ev.ID, 99); !errors.Is(err, sql.ErrNoRows) {
			t.Fatal("retired notice queued", err)
		}
		if err = s.SetState(ctx, ev.ID, 99, "pending"); !errors.Is(err, sql.ErrNoRows) {
			t.Fatal("retired rule reopened", err)
		}
		if _, err = s.RestrictIPWhitelist(ctx, ev.ID, ev.Generation, []string{"198.51.100.0/24"}); !errors.Is(err, sql.ErrNoRows) {
			t.Fatal("retired target accepted", err)
		}
	}
	if err = s.deliver(ctx); err != nil {
		t.Fatal(err)
	}
	if len(m.to) != 0 {
		t.Fatal("retired queued mail sent")
	}
	notices, err := s.UserNotices(ctx, 1, 0)
	if err != nil || len(notices) != 0 {
		t.Fatal("retired rule shown to customer", notices, err)
	}
}
