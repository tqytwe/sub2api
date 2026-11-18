package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type operationsAccount struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Key         string    `json:"key"`
	Attribution string    `json:"attribution"`
	UpdatedAt   time.Time `json:"updated_at"`
	Deleted     bool      `json:"deleted"`
	Schedulable bool      `json:"schedulable"`
	Concurrency int       `json:"concurrency"`
	Status      string    `json:"status"`
}

// The projection never includes credentials. Only the selected endpoint fields
// are read internally, and neither DB errors nor raw URLs are logged/returned.
func operationsAccounts(ctx context.Context, tx *sql.Tx, f service.UpstreamOperationsFilter) ([]byte, error) {
	var grokMode string
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT value FROM settings WHERE key=$1),'')`, service.SettingKeyGrokDefaultBaseURLMode).Scan(&grokMode); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT a.id,a.name,a.platform,a.type,
 jsonb_build_object('base_url',a.credentials->'base_url','api_base_urls',a.credentials->'api_base_urls','account_mode',a.credentials->'account_mode','api_protocol',a.credentials->'api_protocol','provider_profile',a.credentials->'provider_profile','client_id',a.credentials->'client_id','auth_mode',a.credentials->'auth_mode','openai_auth_mode',a.credentials->'openai_auth_mode'),
 jsonb_build_object('custom_base_url_enabled',a.extra->'custom_base_url_enabled','custom_base_url',a.extra->'custom_base_url','openai_siwc',a.extra->'openai_siwc','openai_prism_browser',a.extra->'openai_prism_browser','openai_excel_bps',a.extra->'openai_excel_bps'),
 a.updated_at,a.deleted_at IS NOT NULL,a.parent_account_id,a.schedulable,a.concurrency,a.status
 FROM accounts a WHERE ($3::bigint=0 OR a.id=$3) AND (a.deleted_at IS NULL OR EXISTS(SELECT 1 FROM usage_logs u WHERE u.account_id=a.id AND u.created_at >= $1 AND u.created_at < $2))`, f.From, f.Until, f.AccountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []operationsAccount{}
	for rows.Next() {
		var a service.Account
		var creds, extra []byte
		var deleted bool
		var parent sql.NullInt64
		if err = rows.Scan(&a.ID, &a.Name, &a.Platform, &a.Type, &creds, &extra, &a.UpdatedAt, &deleted, &parent, &a.Schedulable, &a.Concurrency, &a.Status); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(creds, &a.Credentials); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(extra, &a.Extra); err != nil {
			return nil, err
		}
		if parent.Valid {
			a.ParentAccountID = &parent.Int64
		}
		key, attribution := service.ResolveOperationsUpstream(&a, service.GrokBaseURLForMode(grokMode))
		if key == "" {
			key = "unknown:" + attribution
		}
		out = append(out, operationsAccount{ID: a.ID, Name: a.Name, Key: key, Attribution: attribution, UpdatedAt: a.UpdatedAt, Deleted: deleted, Schedulable: a.Schedulable, Concurrency: a.Concurrency, Status: a.Status})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return json.Marshal(out)
}

const upstreamOperationsCohort = `accounts_config AS (
 SELECT * FROM jsonb_to_recordset($3::jsonb) AS c(id bigint,name text,key text,attribution text,updated_at timestamptz,deleted boolean,schedulable boolean,concurrency integer,status text)
), cohort AS (
 SELECT * FROM accounts_config WHERE ($4='' OR key=$4) AND ($5::bigint=0 OR id=$5)
)`

func readUpstreamSection(ctx context.Context, tx *sql.Tx, query string, args ...any) service.ChannelOperationsSection {
	out := service.ChannelOperationsSection{Status: "query_failed", Data: json.RawMessage("null"), QueriedAt: time.Now().UTC(), Error: "Data query failed; retry"}
	if _, err := tx.ExecContext(ctx, "SAVEPOINT operations_section"); err != nil {
		return out
	}
	var raw []byte
	if err := tx.QueryRowContext(ctx, query, args...).Scan(&raw); err != nil {
		_, _ = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT operations_section")
		_, _ = tx.ExecContext(ctx, "RELEASE SAVEPOINT operations_section")
		slog.WarnContext(ctx, "upstream operations query failed", "error_type", fmt.Sprintf("%T", err))
		return out
	}
	if _, err := tx.ExecContext(ctx, "RELEASE SAVEPOINT operations_section"); err != nil {
		return out
	}
	out.Status = "ready"
	out.Error = ""
	out.Data = raw
	return out
}

const upstreamOperationsErrors = `WITH ` + upstreamOperationsCohort + `, params AS (SELECT $8::text AS model_scope), scoped AS (
 SELECT e.error_type,e.error_phase,e.status_code,e.upstream_status_code,e.created_at FROM ops_error_logs e
 WHERE e.created_at >= $1 AND e.created_at < $2 AND COALESCE(e.is_count_tokens,false)=false
 AND ($4='' OR e.account_id IN (SELECT id FROM cohort)) AND ($5::bigint=0 OR e.account_id=$5) AND ($6::bigint=0 OR e.group_id=$6)
 AND ($7='' OR CASE $8 WHEN 'public' THEN COALESCE(NULLIF(e.requested_model,''),e.model) WHEN 'recorded' THEN e.model WHEN 'upstream' THEN e.upstream_model END=$7)
), buckets AS (SELECT error_type,error_phase,status_code,count(*) AS records FROM scoped GROUP BY error_type,error_phase,status_code)
 SELECT jsonb_build_object('source','retained ops_error_logs; current account cohort; count_tokens excluded; no complete request failure denominator','records',(SELECT count(*) FROM scoped),'upstream_5xx',(SELECT count(*) FROM scoped WHERE upstream_status_code>=500),'latest_sample_at',(SELECT max(created_at) FROM scoped),'rows',COALESCE((SELECT jsonb_agg(to_jsonb(b)) FROM (SELECT * FROM buckets ORDER BY records DESC,error_type,error_phase,status_code LIMIT 20)b),'[]'::jsonb),'bucket_total',(SELECT count(*) FROM buckets),'error_rate',NULL)`
