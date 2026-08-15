CREATE TABLE IF NOT EXISTS mobile_app_releases (
    id BIGSERIAL PRIMARY KEY,
    distribution VARCHAR(16) NOT NULL,
    package_name VARCHAR(255) NOT NULL,
    artifact_type VARCHAR(16) NOT NULL,
    version VARCHAR(64) NOT NULL,
    version_code BIGINT NOT NULL,
    storage_key TEXT NOT NULL,
    download_url TEXT NOT NULL DEFAULT '',
    bytes BIGINT NOT NULL,
    sha256 VARCHAR(64) NOT NULL,
    signing_certificate_sha256 VARCHAR(64) NOT NULL,
    min_supported_version_code BIGINT NOT NULL DEFAULT 0,
    notes JSONB NOT NULL DEFAULT '[]'::jsonb,
    notes_i18n JSONB NOT NULL DEFAULT '{}'::jsonb,
    manifest JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(16) NOT NULL DEFAULT 'ready',
    rollout_percent INTEGER NOT NULL DEFAULT 100,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    CONSTRAINT mobile_app_releases_distribution_check CHECK (distribution IN ('direct', 'play')),
    CONSTRAINT mobile_app_releases_artifact_type_check CHECK (artifact_type IN ('apk', 'aab')),
    CONSTRAINT mobile_app_releases_status_check CHECK (status IN ('draft', 'ready', 'published', 'paused', 'retired')),
    CONSTRAINT mobile_app_releases_version_code_check CHECK (version_code > 0),
    CONSTRAINT mobile_app_releases_bytes_check CHECK (bytes >= 0),
    CONSTRAINT mobile_app_releases_rollout_check CHECK (rollout_percent BETWEEN 0 AND 100),
    CONSTRAINT mobile_app_releases_distribution_artifact_check CHECK (
        (distribution = 'direct' AND artifact_type = 'apk') OR
        (distribution = 'play' AND artifact_type = 'aab')
    ),
    CONSTRAINT mobile_app_releases_sha256_check CHECK (sha256 ~ '^[a-f0-9]{64}$'),
    CONSTRAINT mobile_app_releases_signer_check CHECK (signing_certificate_sha256 ~ '^[a-f0-9]{64}$'),
    CONSTRAINT mobile_app_releases_version_unique UNIQUE (distribution, version_code)
);

CREATE INDEX IF NOT EXISTS mobile_app_releases_published_idx
    ON mobile_app_releases (distribution, status, version_code DESC);
