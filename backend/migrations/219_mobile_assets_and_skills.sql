-- Mobile material library and server-managed skill center.

CREATE TABLE IF NOT EXISTS mobile_assets (
    id UUID PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind VARCHAR(32) NOT NULL,
    source VARCHAR(32) NOT NULL,
    storage_key VARCHAR(1024) NOT NULL,
    original_name VARCHAR(255) NOT NULL,
    content_type VARCHAR(255) NOT NULL,
    byte_size BIGINT NOT NULL DEFAULT 0,
    sha256 VARCHAR(64),
    status VARCHAR(32) NOT NULL DEFAULT 'uploading',
    source_type VARCHAR(64),
    source_id VARCHAR(128),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT mobile_assets_kind_check CHECK (kind IN ('image', 'audio', 'video', 'pdf', 'document', 'file')),
    CONSTRAINT mobile_assets_source_check CHECK (source IN ('upload', 'share', 'image_result', 'chat_export', 'voice')),
    CONSTRAINT mobile_assets_status_check CHECK (status IN ('uploading', 'ready', 'failed', 'deleted')),
    CONSTRAINT mobile_assets_byte_size_check CHECK (byte_size >= 0)
);

CREATE INDEX IF NOT EXISTS idx_mobile_assets_user_created ON mobile_assets (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_mobile_assets_user_kind_created ON mobile_assets (user_id, kind, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_mobile_assets_user_status_created ON mobile_assets (user_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_mobile_assets_deleted_at ON mobile_assets (deleted_at);
CREATE INDEX IF NOT EXISTS idx_mobile_assets_source ON mobile_assets (source_type, source_id)
    WHERE source_type IS NOT NULL AND source_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_mobile_assets_sha256 ON mobile_assets (user_id, sha256)
    WHERE deleted_at IS NULL AND sha256 IS NOT NULL;

CREATE TABLE IF NOT EXISTS mobile_skills (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    slug VARCHAR(100) NOT NULL UNIQUE,
    status VARCHAR(16) NOT NULL DEFAULT 'draft',
    name_zh VARCHAR(120) NOT NULL,
    description_zh TEXT NOT NULL DEFAULT '',
    category VARCHAR(80) NOT NULL DEFAULT '',
    icon_url TEXT NOT NULL DEFAULT '',
    cover_url TEXT NOT NULL DEFAULT '',
    current_version INTEGER NOT NULL DEFAULT 1,
    published_version INTEGER,
    featured BOOLEAN NOT NULL DEFAULT false,
    sort_order INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT mobile_skills_status_check CHECK (status IN ('draft', 'published', 'offline')),
    CONSTRAINT mobile_skills_version_check CHECK (current_version > 0 AND (published_version IS NULL OR published_version > 0))
);

CREATE INDEX IF NOT EXISTS idx_mobile_skills_status_sort ON mobile_skills (status, sort_order);
CREATE INDEX IF NOT EXISTS idx_mobile_skills_category_status ON mobile_skills (category, status);
CREATE INDEX IF NOT EXISTS idx_mobile_skills_featured_status ON mobile_skills (featured, status);

CREATE TABLE IF NOT EXISTS mobile_skill_versions (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    skill_id BIGINT NOT NULL REFERENCES mobile_skills(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    prompt_id BIGINT NOT NULL DEFAULT 0,
    prompt_version INTEGER NOT NULL DEFAULT 1,
    system_prompt_override TEXT,
    input_schema JSONB NOT NULL DEFAULT '{}'::jsonb,
    examples JSONB NOT NULL DEFAULT '[]'::jsonb,
    tool_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    model_policy JSONB NOT NULL DEFAULT '{}'::jsonb,
    consumption_note_zh TEXT NOT NULL DEFAULT '',
    changelog_zh TEXT NOT NULL DEFAULT '',
    CONSTRAINT mobile_skill_versions_version_check CHECK (version > 0 AND prompt_version > 0),
    UNIQUE (skill_id, version)
);

CREATE INDEX IF NOT EXISTS idx_mobile_skill_versions_prompt ON mobile_skill_versions (prompt_id, prompt_version);

CREATE TABLE IF NOT EXISTS user_mobile_skills (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    skill_id BIGINT NOT NULL REFERENCES mobile_skills(id) ON DELETE CASCADE,
    installed_version INTEGER NOT NULL,
    pinned BOOLEAN NOT NULL DEFAULT false,
    last_used_at TIMESTAMPTZ,
    CONSTRAINT user_mobile_skills_version_check CHECK (installed_version > 0),
    UNIQUE (user_id, skill_id)
);

CREATE INDEX IF NOT EXISTS idx_user_mobile_skills_user_pinned_used ON user_mobile_skills (user_id, pinned, last_used_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_mobile_skills_skill ON user_mobile_skills (skill_id);

INSERT INTO mobile_skills (slug, status, name_zh, description_zh, category, current_version, published_version, featured, sort_order)
VALUES
    ('general-assistant', 'published', '通用智能助手', '日常问答、信息整理、方案分析与执行建议。', '通用', 1, 1, true, 10),
    ('deep-research', 'published', '深度研究员', '拆解复杂问题，核对证据，标注假设并给出可执行结论。', '研究', 1, 1, true, 20),
    ('writing-editor', 'published', '中文写作编辑', '创作、改写与校对中文内容，保持自然准确的表达。', '写作', 1, 1, true, 30),
    ('coding-engineer', 'published', '软件工程师', '分析代码、定位缺陷、设计实现并给出验证步骤。', '开发', 1, 1, true, 40),
    ('product-operator', 'published', '产品运营顾问', '面向真实用户设计产品、活动、增长和运营方案。', '产品', 1, 1, false, 50),
    ('business-analyst', 'published', '商业分析师', '梳理商业问题、成本、风险、指标与决策选项。', '商业', 1, 1, false, 60),
    ('study-tutor', 'published', '学习辅导老师', '按学习目标讲解知识、设计练习并检查理解。', '教育', 1, 1, false, 70),
    ('image-director', 'published', '视觉创作导演', '把创意需求整理成完整、清晰、通用于多种图片模型的创作指令。', '创作', 1, 1, true, 80)
ON CONFLICT (slug) DO UPDATE SET
    status = EXCLUDED.status,
    name_zh = EXCLUDED.name_zh,
    description_zh = EXCLUDED.description_zh,
    category = EXCLUDED.category,
    current_version = EXCLUDED.current_version,
    published_version = EXCLUDED.published_version,
    featured = EXCLUDED.featured,
    sort_order = EXCLUDED.sort_order,
    updated_at = NOW();

INSERT INTO mobile_skill_versions
    (skill_id, version, prompt_id, prompt_version, system_prompt_override, input_schema, examples, tool_config, model_policy, consumption_note_zh, changelog_zh)
SELECT id, 1, 0, 1,
    CASE slug
        WHEN 'general-assistant' THEN '你是 Jisudeng 通用智能助手。先理解用户真正要解决的问题，再给出准确、清晰、可执行的回答。信息不足时明确假设，不编造事实。'
        WHEN 'deep-research' THEN '你是严谨的深度研究员。拆解问题，区分事实、推断与未知，核对关键证据和时间范围，比较可行方案，最后给出有依据的结论与行动清单。'
        WHEN 'writing-editor' THEN '你是中文写作编辑。保留用户原意和语气，优化结构、逻辑、准确性与可读性。除非用户要求，不堆砌术语，不使用生硬翻译腔。'
        WHEN 'coding-engineer' THEN '你是资深软件工程师。先理解现有系统和约束，再定位根因、设计最小可靠改动、考虑兼容性与安全，并提供可以复现的验证方法。'
        WHEN 'product-operator' THEN '你是产品运营顾问。围绕目标用户、核心场景、体验、成本和可衡量结果提出方案，明确优先级、依赖、风险和验收标准。'
        WHEN 'business-analyst' THEN '你是商业分析师。用结构化方法分析收入、成本、竞争、风险和关键指标，明确数据缺口，给出可比较的决策选项。'
        WHEN 'study-tutor' THEN '你是耐心的学习辅导老师。根据用户水平分步解释，用例子检查理解，指出常见误区，并提供难度合适的练习与反馈。'
        WHEN 'image-director' THEN '你是视觉创作导演。将用户目标整理成完整的视觉创作指令，明确主体、环境、构图、镜头、光线、色彩、材质、风格、文字要求和排除项。不得绑定或自动切换任何具体图片模型。'
    END,
    '{"type":"object","properties":{"request":{"type":"string","title":"需求"}},"required":["request"]}'::jsonb,
    '[]'::jsonb,
    '{}'::jsonb,
    '{"model":"current","allow_model_switch":false}'::jsonb,
    '按当前所选模型和分组的实际用量计费，使用前不会自动切换模型。',
    '首个正式版本。'
FROM mobile_skills
WHERE slug IN ('general-assistant','deep-research','writing-editor','coding-engineer','product-operator','business-analyst','study-tutor','image-director')
ON CONFLICT (skill_id, version) DO UPDATE SET
    system_prompt_override = EXCLUDED.system_prompt_override,
    input_schema = EXCLUDED.input_schema,
    model_policy = EXCLUDED.model_policy,
    consumption_note_zh = EXCLUDED.consumption_note_zh,
    changelog_zh = EXCLUDED.changelog_zh,
    updated_at = NOW();
