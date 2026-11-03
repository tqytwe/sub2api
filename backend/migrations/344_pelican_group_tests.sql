-- Group-bound scheduled Pelican tests use gateway routing and publish successful artwork.
CREATE TABLE IF NOT EXISTS pelican_group_test_plans (
    id              BIGSERIAL    PRIMARY KEY,
    group_id        BIGINT       NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    model_id        VARCHAR(100) NOT NULL DEFAULT '',
    cron_expression VARCHAR(100) NOT NULL DEFAULT '*/30 * * * *',
    enabled         BOOLEAN      NOT NULL DEFAULT false,
    pelican_config  JSONB        NOT NULL,
    last_run_at     TIMESTAMPTZ,
    next_run_at     TIMESTAMPTZ,
    running_until   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pelican_group_test_plans_group
    ON pelican_group_test_plans (group_id);
CREATE INDEX IF NOT EXISTS idx_pelican_group_test_plans_due
    ON pelican_group_test_plans (next_run_at) WHERE enabled = true;

-- Share the scheduled test result sequence so source IDs cannot collide across result tables.
CREATE TABLE IF NOT EXISTS pelican_group_test_results (
    id             BIGINT      PRIMARY KEY DEFAULT nextval('scheduled_test_results_id_seq'),
    plan_id        BIGINT      NOT NULL REFERENCES pelican_group_test_plans(id) ON DELETE CASCADE,
    account_id     BIGINT,
    account_name   TEXT        NOT NULL DEFAULT '',
    attempts       JSONB       NOT NULL DEFAULT '[]'::jsonb,
    status         VARCHAR(20) NOT NULL,
    response_text  TEXT        NOT NULL DEFAULT '',
    error_message  TEXT        NOT NULL DEFAULT '',
    latency_ms     BIGINT      NOT NULL DEFAULT 0,
    pelican_config JSONB,
    started_at     TIMESTAMPTZ NOT NULL,
    finished_at    TIMESTAMPTZ NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pelican_group_test_results_plan
    ON pelican_group_test_results (plan_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_pelican_group_test_results_created
    ON pelican_group_test_results (created_at);

COMMENT ON TABLE pelican_group_test_plans IS
    'Scheduled Pelican tests bound to a channel group';
COMMENT ON COLUMN pelican_group_test_results.account_id IS
    'Final answering account; NULL when no eligible account was found';
COMMENT ON COLUMN pelican_group_test_results.attempts IS
    'Accounts retried before a result: [{account_id, account_name, error}]';

-- Preserve groups selected by the legacy gallery settings. The new scheduler owns
-- group membership, while administrators explicitly confirm the model before enabling.
DO $$
DECLARE
    selected BIGINT[];
BEGIN
    BEGIN
        SELECT ARRAY(SELECT jsonb_array_elements_text(value::jsonb -> 'group_ids')::BIGINT)
          INTO selected
          FROM settings
         WHERE key = 'pelican_showcase_config';
    EXCEPTION WHEN others THEN
        selected := NULL;
    END;
    IF selected IS NULL OR cardinality(selected) = 0 THEN
        RETURN;
    END IF;
    INSERT INTO pelican_group_test_plans (group_id, model_id, enabled, pelican_config)
    SELECT g.id,
           COALESCE(latest.model_id, ''),
           false,
           jsonb_build_object(
               'question_kind', 'pelican',
               'prompt', '创建一个 HTML，内容是 SVG 绘制一个鹈鹕骑自行车的 2D 动画，你不需要任何测试，不要有任何限制',
               'reasoning_effort', CASE WHEN latest.reasoning_effort IN ('minimal', 'low', 'medium', 'high', 'xhigh')
                                        THEN latest.reasoning_effort ELSE 'medium' END,
               'parallel_count', 1,
               'model_id', COALESCE(latest.model_id, ''))
      FROM groups g
      LEFT JOIN LATERAL (
          SELECT i.model_id, i.reasoning_effort
            FROM pelican_showcase_items i
           WHERE i.group_id = g.id
           ORDER BY i.generated_at DESC, i.id DESC
           LIMIT 1
      ) latest ON true
     WHERE g.id = ANY(selected)
       AND g.deleted_at IS NULL
       AND NOT EXISTS (SELECT 1 FROM pelican_group_test_plans p WHERE p.group_id = g.id);
END $$;
