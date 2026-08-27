-- Project-scoped short-drama creation facts.  A project is the durable root
-- for its episodes, five creator-facing documents, and links to private
-- mobile_assets.  No provider credential, production confirmation, billing
-- reservation, or composition artifact belongs in this foundation.

CREATE TABLE IF NOT EXISTS studio_projects (
    id UUID PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(200) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    aspect_ratio VARCHAR(16) NOT NULL DEFAULT '9:16',
    language VARCHAR(16) NOT NULL DEFAULT 'zh-CN',
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    cover_asset_id UUID REFERENCES mobile_assets(id) ON DELETE SET NULL,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ,
    CONSTRAINT studio_projects_title_check CHECK (length(btrim(title)) > 0),
    CONSTRAINT studio_projects_aspect_ratio_check CHECK (aspect_ratio IN ('9:16', '16:9', '1:1')),
    CONSTRAINT studio_projects_status_check CHECK (status IN ('draft', 'active', 'archived')),
    CONSTRAINT studio_projects_version_check CHECK (version > 0)
);

CREATE INDEX IF NOT EXISTS idx_studio_projects_user_updated
    ON studio_projects (user_id, updated_at DESC, id DESC)
    WHERE archived_at IS NULL;

CREATE TABLE IF NOT EXISTS studio_episodes (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES studio_projects(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    stable_id VARCHAR(64) NOT NULL,
    title VARCHAR(200) NOT NULL,
    sequence INTEGER NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT studio_episodes_stable_id_check CHECK (length(btrim(stable_id)) > 0),
    CONSTRAINT studio_episodes_title_check CHECK (length(btrim(title)) > 0),
    CONSTRAINT studio_episodes_sequence_check CHECK (sequence > 0),
    CONSTRAINT studio_episodes_status_check CHECK (status IN ('draft', 'ready', 'archived')),
    UNIQUE (project_id, stable_id),
    UNIQUE (project_id, sequence)
);

CREATE INDEX IF NOT EXISTS idx_studio_episodes_project_sequence
    ON studio_episodes (project_id, sequence, id);
CREATE INDEX IF NOT EXISTS idx_studio_episodes_user_project
    ON studio_episodes (user_id, project_id);

CREATE TABLE IF NOT EXISTS studio_documents (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES studio_projects(id) ON DELETE CASCADE,
    episode_id UUID REFERENCES studio_episodes(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    document_type VARCHAR(32) NOT NULL,
    content JSONB NOT NULL DEFAULT '{}'::jsonb,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMPTZ,
    CONSTRAINT studio_documents_type_check CHECK (document_type IN ('script', 'visual_bible', 'storyboard', 'image_prompts', 'video_prompts')),
    CONSTRAINT studio_documents_version_check CHECK (version > 0),
    UNIQUE (project_id, episode_id, document_type, version)
);

-- Project documents have no episode_id; PostgreSQL's NULL uniqueness semantics
-- need a partial index to make each version unambiguous at project scope.
CREATE UNIQUE INDEX IF NOT EXISTS uq_studio_project_document_version
    ON studio_documents (project_id, document_type, version)
    WHERE episode_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_studio_documents_project_episode_type
    ON studio_documents (project_id, episode_id, document_type, version DESC)
    WHERE archived_at IS NULL;

CREATE TABLE IF NOT EXISTS studio_asset_links (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES studio_projects(id) ON DELETE CASCADE,
    episode_id UUID REFERENCES studio_episodes(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    asset_id UUID NOT NULL REFERENCES mobile_assets(id) ON DELETE RESTRICT,
    link_type VARCHAR(32) NOT NULL,
    stable_ref VARCHAR(64),
    position INTEGER,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT studio_asset_links_type_check CHECK (link_type IN ('reference', 'character', 'scene', 'prop', 'keyframe', 'shot', 'video', 'audio', 'final')),
    CONSTRAINT studio_asset_links_position_check CHECK (position IS NULL OR position >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_studio_asset_links_identity
    ON studio_asset_links (project_id, asset_id, link_type, COALESCE(stable_ref, ''));
CREATE INDEX IF NOT EXISTS idx_studio_asset_links_project_episode
    ON studio_asset_links (project_id, episode_id, position, created_at);
CREATE INDEX IF NOT EXISTS idx_studio_asset_links_asset
    ON studio_asset_links (asset_id);
