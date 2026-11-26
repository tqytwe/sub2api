//go:build integration

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

type profitChargeFixture struct {
	t                                     *testing.T
	db                                    *sql.DB
	r                                     *channelRepository
	pid, account, user, key, channel, seq int64
	base, model                           string
	epoch                                 time.Time
}

var profitFixtureDatabases = struct {
	sync.Mutex
	byTest map[*testing.T]*sql.DB
}{byTest: make(map[*testing.T]*sql.DB)}

func profitFixtureDB(t *testing.T) *sql.DB {
	t.Helper()
	profitFixtureDatabases.Lock()
	defer profitFixtureDatabases.Unlock()
	if db := profitFixtureDatabases.byTest[t]; db != nil {
		return db
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	database := "sub2api_test_profit_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quotedDatabase := pq.QuoteIdentifier(database)
	_, err := integrationDB.ExecContext(ctx, "CREATE DATABASE "+quotedDatabase+" TEMPLATE template0")
	require.NoError(t, err)
	var db *sql.DB
	t.Cleanup(func() {
		if db != nil {
			require.NoError(t, db.Close())
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, cleanupErr := integrationDB.ExecContext(cleanupCtx, "DROP DATABASE "+quotedDatabase)
		require.NoError(t, cleanupErr)
		profitFixtureDatabases.Lock()
		delete(profitFixtureDatabases.byTest, t)
		profitFixtureDatabases.Unlock()
	})

	endpoint, err := url.Parse(integrationDSN)
	require.NoError(t, err)
	endpoint.Path = "/" + database
	db, err = sql.Open("postgres", endpoint.String())
	require.NoError(t, err)
	require.NoError(t, db.PingContext(ctx))
	require.NoError(t, ApplyMigrations(ctx, db))
	profitFixtureDatabases.byTest[t] = db
	return db
}

func TestProviderProfitFixtureUsesUniqueSiteAndModel(t *testing.T) {
	first := newProfitChargeFixture(t)
	second := newProfitChargeFixture(t)
	at := first.now()
	firstUsage := first.usage(first.account, "fixture-isolation-first", at)
	secondUsage := second.usage(second.account, "fixture-isolation-second", at)

	var firstBase, secondBase, firstModel, secondModel string
	require.NoError(t, first.db.QueryRow(`SELECT base_url FROM mature_providers WHERE id=$1`, first.pid).Scan(&firstBase))
	require.NoError(t, second.db.QueryRow(`SELECT base_url FROM mature_providers WHERE id=$1`, second.pid).Scan(&secondBase))
	require.NoError(t, first.db.QueryRow(`SELECT model FROM usage_logs WHERE id=$1`, firstUsage).Scan(&firstModel))
	require.NoError(t, second.db.QueryRow(`SELECT model FROM usage_logs WHERE id=$1`, secondUsage).Scan(&secondModel))
	require.NotEqual(t, firstBase, secondBase, "provider-site evidence must not collide across test fixtures")
	require.NotEqual(t, firstModel, secondModel, "report filters must not include stale rows from another fixture")
}

func newProfitChargeFixture(t *testing.T) *profitChargeFixture {
	t.Helper()
	db := profitFixtureDB(t)
	suffix := uuid.NewString()
	f := &profitChargeFixture{
		t: t, db: db, r: &channelRepository{db: db},
		base:  "https://profit-fixture-" + suffix + ".invalid",
		model: "fixture-" + suffix,
	}
	f.pid = f.provider()
	f.channel = f.id(`SELECT channel_id FROM provider_profit_links WHERE provider_id=$1`, f.pid)
	var err error
	f.epoch, err = f.r.ProviderProfitIdentityFrom(context.Background(), f.pid)
	require.NoError(t, err)
	f.account = f.imported(f.pid, 11)
	f.user = f.id(`INSERT INTO users(email,password_hash,balance) VALUES($1,'synthetic',23) RETURNING id`, "reconcile+"+suffix+"@test.invalid")
	f.key = f.id(`INSERT INTO api_keys(user_id,key,name) VALUES($1,$2,'fixture') RETURNING id`, f.user, "local-synthetic-"+suffix)
	return f
}
func (f *profitChargeFixture) id(q string, args ...any) int64 {
	f.t.Helper()
	var id int64
	require.NoError(f.t, f.db.QueryRow(q, args...).Scan(&id))
	return id
}
func (f *profitChargeFixture) exec(q string, args ...any) {
	f.t.Helper()
	_, err := f.db.Exec(q, args...)
	require.NoError(f.t, err)
}
func (f *profitChargeFixture) provider() int64 {
	pid := f.id(`INSERT INTO mature_providers(name,site_type,base_url) VALUES('fixture','sub2api',$1) RETURNING id`, f.base)
	f.exec(`UPDATE provider_profit_links SET cost_mode='wallet',revision=revision+1 WHERE provider_id=$1`, pid)
	return pid
}
func (f *profitChargeFixture) imported(provider, key int64) int64 {
	return f.id(`INSERT INTO accounts(name,type,platform,extra,credentials) VALUES('fixture','apikey','openai',jsonb_build_object('provider_binding',jsonb_build_object('provider_id',$1::bigint,'key_id',$2::text,'provider_revision',(SELECT revision FROM mature_providers WHERE id=$1))),jsonb_build_object('base_url',(SELECT rtrim(base_url,'/') FROM mature_providers WHERE id=$1),'api_key','fixture-only')) RETURNING id`, provider, fmt.Sprint(key))
}
func (f *profitChargeFixture) now() time.Time {
	f.t.Helper()
	var at time.Time
	require.NoError(f.t, f.db.QueryRow(`SELECT clock_timestamp()`).Scan(&at))
	return at
}
func (f *profitChargeFixture) usage(acct int64, up any, at time.Time) int64 {
	return f.usageWithIncome(acct, up, at, 1)
}
func (f *profitChargeFixture) usageWithIncome(acct int64, up any, at time.Time, income int) int64 {
	f.seq++
	return f.id(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,upstream_request_id,model,actual_cost,total_cost,created_at) VALUES($1,$2,$3,$4,$5,$8,$7,12,$6) RETURNING id`, f.user, f.key, acct, fmt.Sprintf("local-bill-%d", f.seq), up, at, income, f.model)
}
func profitRemote(id, key int64, req string, at time.Time) service.ProviderProfitRemoteCharge {
	return service.ProviderProfitRemoteCharge{UsageID: id, UserID: 4, KeyID: key, RequestID: req, CreatedAt: at, Amount: "0.000123456789123456789"}
}
func (f *profitChargeFixture) sync(w service.ProviderProfitWindow, rows ...service.ProviderProfitRemoteCharge) (service.ProviderProfitSyncResult, error) {
	return f.r.ReconcileProviderProfitCharges(context.Background(), f.pid, f.epoch, f.base, w, rows)
}
func (f *profitChargeFixture) counts(w service.ProviderProfitWindow, m, u int, rows ...service.ProviderProfitRemoteCharge) {
	f.t.Helper()
	out, err := f.sync(w, rows...)
	require.NoError(f.t, err)
	require.Equal(f.t, service.ProviderProfitSyncResult{Matched: m, Unmatched: u}, out)
	require.EqualValues(f.t, 1, f.id(`SELECT count(*) FROM provider_profit_sync_runs WHERE provider_id=$1 AND window_from=$2 AND window_until=$3`, f.pid, w.From, w.Until))
	require.EqualValues(f.t, m, f.id(`SELECT matched FROM provider_profit_sync_runs WHERE provider_id=$1 AND window_from=$2 AND window_until=$3`, f.pid, w.From, w.Until))
}
func (f *profitChargeFixture) snapshot(q string) string {
	f.t.Helper()
	var s string
	require.NoError(f.t, f.db.QueryRow(q).Scan(&s))
	return s
}

func (f *profitChargeFixture) providerLink() service.ProviderProfitLink {
	f.t.Helper()
	for page := 1; ; page++ {
		links, err := f.r.ListProviderProfitLinks(context.Background(), page, 100)
		require.NoError(f.t, err)
		for _, link := range links.Items {
			if link.ProviderID == f.pid {
				return link
			}
		}
		if int64(page*100) >= links.Total {
			break
		}
	}
	f.t.Fatalf("provider profit link %d not found", f.pid)
	return service.ProviderProfitLink{}
}

func TestProviderProfitChargesRequireUniqueImportedEvidence(t *testing.T) {
	f := newProfitChargeFixture(t)
	second := f.imported(f.pid, 11)
	different := f.imported(f.pid, 12)
	alias := f.imported(f.provider(), 11)
	at := f.now().Add(10 * time.Minute)
	w := service.ProviderProfitWindow{From: at, Until: at.Add(15 * time.Minute)}
	ok := f.usage(f.account, "exact", at)
	f.usage(different, "exact", at)
	f.usage(f.account, "duplicate", at)
	f.usage(second, "duplicate", at)
	f.usage(f.account, "edge-duplicate", at)
	f.usage(second, "edge-duplicate", at.Add(-time.Microsecond))
	f.usage(f.account, "alias-duplicate", at)
	f.usage(alias, "alias-duplicate", at)
	f.usage(f.account, nil, at)
	f.usage(f.account, "prefix", at)
	f.usage(f.account, "wrong-key", at)
	f.usage(f.account, "skew", at)
	f.usage(f.account, "end", w.Until)
	// The eleventh row in the requested window has no upstream ID. Its own
	// billing request_id matching the upstream (by design) is insufficient.
	f.usage(f.account, nil, at)
	rows := []service.ProviderProfitRemoteCharge{
		profitRemote(1, 11, "exact", at), profitRemote(2, 12, "exact", at), profitRemote(3, 11, "duplicate", at), profitRemote(4, 11, "edge-duplicate", at), profitRemote(5, 11, "alias-duplicate", at), profitRemote(6, 11, "client:prefix", at), profitRemote(7, 12, "wrong-key", at), profitRemote(8, 11, "skew", at.Add(service.ProviderProfitClockSkew+time.Microsecond)), profitRemote(9, 11, "local-bill-14", at), profitRemote(10, 11, "unrelated-wallet-activity", at),
	}
	const billing = `SELECT jsonb_build_array((SELECT jsonb_agg(to_jsonb(u) ORDER BY id) FROM usage_logs u),(SELECT jsonb_agg(to_jsonb(k) ORDER BY id) FROM api_keys k),(SELECT jsonb_agg(to_jsonb(u) ORDER BY id) FROM users u),(SELECT count(*) FROM usage_billing_quotes))::text`
	before := f.snapshot(billing)
	f.counts(w, 2, 9, rows...)
	f.counts(w, 2, 9, rows...)
	require.EqualValues(t, 2, f.id(`SELECT count(*) FROM provider_profit_charges WHERE provider_id=$1`, f.pid))
	require.Equal(t, before, f.snapshot(billing))
	var raw string
	require.NoError(t, f.db.QueryRow(`SELECT to_jsonb(c)::text FROM provider_profit_charges c WHERE usage_log_id=$1`, ok).Scan(&raw))
	for _, secret := range []string{"fixture-only", "profit-fixture.invalid", "exact", "local-bill"} {
		require.NotContains(t, raw, secret)
	}
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(raw), &fields))
	require.Equal(t, "null", string(fields["settled_amount"]))
	require.Equal(t, `"reported_usage"`, string(fields["evidence_kind"]))
	require.Equal(t, "0.000123456789123456789", string(fields["amount"]))
	// PostgreSQL instants are microsecond precise; a submicrosecond start
	// excludes exactly-at, and a submicrosecond end still includes it.
	f.counts(service.ProviderProfitWindow{From: at.Add(time.Nanosecond), Until: at.Add(time.Microsecond)}, 0, 0, rows...)
	f.counts(service.ProviderProfitWindow{From: at, Until: at.Add(time.Nanosecond)}, 2, 9, rows...)
}

func TestProviderProfitChargesLargeWindowAtomicReplay(t *testing.T) {
	f := newProfitChargeFixture(t)
	const total = 5207
	at := f.now()
	w := service.ProviderProfitWindow{From: at, Until: at.Add(time.Hour)}
	f.exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,upstream_request_id,model,actual_cost,total_cost,created_at)
		SELECT $1,$2,$3,'local-'||i,'synthetic-bill-'||i,$6,1,12,$4 FROM generate_series(1,$5::int) AS i ORDER BY i`, f.user, f.key, f.account, at, total, f.model)
	remote := make([]service.ProviderProfitRemoteCharge, total)
	for i := range remote {
		remote[i] = profitRemote(int64(i+1), 11, fmt.Sprintf("synthetic-bill-%d", i+1), at)
		remote[i].Amount = "0.125"
	}
	// The first bill has two local candidates, farther apart than the skew.
	// Only a whole-window ambiguity check rejects the first one. A second
	// collision in the upper skew margin is not itself counted as unmatched.
	f.usage(f.account, remote[0].RequestID, at.Add(time.Minute*20))
	f.usage(f.account, remote[1].RequestID, w.Until)
	f.usage(f.account, nil, at)
	last := &remote[total-1]
	const matched, unmatched = total - 2, 4
	// Freeze one good early match and the last. The full bill then promotes
	// the first and inserts thousands of new rows before the last conflicts.
	f.counts(w, 2, total, remote[2], *last)
	debit := "0.1"
	remote[2].SettledAmount = &debit
	last.Amount = "9"
	out, err := f.sync(w, remote...)
	require.ErrorIs(t, err, service.ErrMatureProviderConflict)
	require.Zero(t, out)
	require.EqualValues(t, 2, f.id(`SELECT count(*) FROM provider_profit_charges WHERE provider_id=$1`, f.pid))
	require.Zero(t, f.id(`SELECT count(settled_amount) FROM provider_profit_charges WHERE provider_id=$1`, f.pid))
	require.EqualValues(t, 1, f.id(`SELECT count(*) FROM provider_profit_sync_runs WHERE provider_id=$1`, f.pid))
	require.EqualValues(t, 2, f.id(`SELECT matched FROM provider_profit_sync_runs WHERE provider_id=$1`, f.pid))

	last.Amount = "0.125"
	for range 2 {
		f.counts(w, matched, unmatched, remote...)
		require.EqualValues(t, matched, f.id(`SELECT count(*) FROM provider_profit_charges WHERE provider_id=$1`, f.pid))
		require.EqualValues(t, 1, f.id(`SELECT count(*) FROM provider_profit_charges WHERE provider_id=$1 AND settled_amount=0.1`, f.pid))
		require.Equal(t, "650.625", f.snapshot(`SELECT sum(amount)::text FROM provider_profit_charges WHERE provider_id=`+fmt.Sprint(f.pid)))
		require.Zero(t, f.id(`SELECT count(*) FROM provider_profit_charges WHERE provider_id=$1 AND upstream_usage_id IN (1,2)`, f.pid))
	}
}

func TestProviderProfitChargesFreezeOwnershipAndPromoteOnlyMatchingDebit(t *testing.T) {
	f := newProfitChargeFixture(t)
	at := f.now()
	w := service.ProviderProfitWindow{From: at, Until: at.Add(time.Hour)}
	a := f.usage(f.account, "private-observation-a", at)
	b := f.usage(f.account, "private-observation-b", at)
	ra := profitRemote(100, 11, "private-observation-a", at)
	rb := profitRemote(101, 11, "private-observation-b", at)
	f.counts(w, 1, 1, ra)
	f.counts(w, 1, 1, ra)
	// Return different evidence about a previously frozen row together with
	// a new valid row. The entire statement must roll back, including b.
	bad := ra
	bad.Amount = "9"
	_, err := f.sync(w, bad, rb)
	require.ErrorIs(t, err, service.ErrMatureProviderConflict)
	require.Zero(t, f.id(`SELECT count(*) FROM provider_profit_charges WHERE usage_log_id=$1`, b))
	debit := "0.00007813"
	ra.SettledAmount = &debit
	f.counts(w, 2, 0, ra, rb)
	f.counts(w, 2, 0, ra, rb)
	check := func() {
		t.Helper()
		var amount, settled, kind string
		var channel, account, provider int64
		require.NoError(t, f.db.QueryRow(`SELECT amount::text,settled_amount::text,evidence_kind,channel_id,account_id,provider_id FROM provider_profit_charges WHERE usage_log_id=$1`, a).Scan(&amount, &settled, &kind, &channel, &account, &provider))
		require.Equal(t, ra.Amount, amount)
		require.Equal(t, debit, settled)
		require.Equal(t, "settled_debit", kind)
		require.Equal(t, []int64{f.channel, f.account, f.pid}, []int64{channel, account, provider})
	}
	check()
	bad = ra
	mismatch := "0.00007812"
	bad.SettledAmount = &mismatch
	_, err = f.sync(w, bad)
	require.ErrorIs(t, err, service.ErrMatureProviderConflict)
	check()
	ra.SettledAmount = nil
	f.counts(w, 1, 1, ra)
	check() // absence of proof on a later lookup does not erase the first one
	for _, q := range []string{`UPDATE provider_profit_charges SET settled_amount=NULL WHERE usage_log_id=$1`, `UPDATE provider_profit_charges SET provider_id=provider_id+1 WHERE usage_log_id=$1`, `DELETE FROM provider_profit_charges WHERE usage_log_id=$1`} {
		_, err := f.db.Exec(q, a)
		require.ErrorContains(t, err, "immutable")
	}
	// Relinking this provider changes only future ownership. The same evidence
	// is attributed to its original channel even when replayed after the edit.
	other := f.id(`INSERT INTO upstream_channels(code,name) VALUES($1,$2) RETURNING id`, f.model+"-relink", f.model+" relink")
	f.exec(`UPDATE provider_profit_links SET channel_id=$1, revision=revision+1 WHERE provider_id=$2`, other, f.pid)
	f.counts(w, 2, 0, ra, rb)
	check()
	// An event before/after a channel move is not one proven owner.
	cross := profitRemote(102, 11, "across-link", f.now())
	f.usage(f.account, cross.RequestID, at)
	f.counts(w, 0, 3, cross)
	f.exec(`UPDATE accounts SET credentials=jsonb_set(credentials,'{api_key}','"other-key"') WHERE id=$1`, f.account)
	unattributed := f.now()
	f.usage(f.account, "after-key-edit", unattributed)
	f.counts(w, 0, 3, profitRemote(103, 11, "after-key-edit", unattributed))
	check()
	// A racing administrative account edit invalidates the captured session,
	// even if its base URL and numeric upstream key IDs still look identical.
	f.exec(`UPDATE mature_providers SET account='other-account', revision=revision+1 WHERE id=$1`, f.pid)
	_, err = f.sync(w, ra)
	require.ErrorIs(t, err, service.ErrMatureProviderConflict)
	f.epoch, err = f.r.ProviderProfitIdentityFrom(context.Background(), f.pid)
	require.NoError(t, err)
	f.counts(w, 0, 3, ra, rb)
	check()
	f.exec(`DELETE FROM mature_providers WHERE id=$1`, f.pid)
	check()
}
