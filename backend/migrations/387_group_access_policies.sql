-- Ported from fixed v2.21.2 migrations 245/246/287/288; local unique number.
-- 用户在单个分组内被禁用的模型清单，与专属倍率 / RPM 共用一行。
-- NULL 表示不限制；rate_multiplier、rpm_override、denied_models 都为 NULL 时整行删除。
ALTER TABLE user_group_rate_multipliers
    ADD COLUMN IF NOT EXISTS denied_models JSONB NULL;

COMMENT ON COLUMN user_group_rate_multipliers.denied_models IS
    '该用户在此分组被禁用的模型（JSON 字符串数组，不区分大小写，支持末尾 * 通配）；NULL 表示不限制';

-- 分组「仅允许流式请求」开关：开启后 Messages / Chat Completions / Responses 的非流式请求
-- 与 Gemini generateContent 在网关入口直接拒绝；默认关闭，行为与之前一致。
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS stream_only BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN groups.stream_only IS '是否仅允许流式请求：开启后非流式的对话生成请求在网关入口直接拒绝';

-- Employee API admission only. Internal account tests and quality judges select
-- accounts directly and do not pass through the public gateway middleware.
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS codex_cli_only BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN groups.codex_cli_only IS
    'Only allow Codex clients at employee API ingress; internal account tests and quality judges are unaffected';

-- Employee ingress policy; existing groups remain unrestricted.
ALTER TABLE groups ADD COLUMN IF NOT EXISTS block_subagents BOOLEAN NOT NULL DEFAULT FALSE;
