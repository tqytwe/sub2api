-- Prompt Library core schema.
-- External source payloads and evidence are durable review records and are never
-- exposed by public APIs.

CREATE TABLE IF NOT EXISTS prompt_categories (
    id              BIGSERIAL PRIMARY KEY,
    slug            VARCHAR(96) NOT NULL UNIQUE,
    name_zh         VARCHAR(128) NOT NULL,
    description_zh  TEXT NOT NULL DEFAULT '',
    dimension       VARCHAR(24) NOT NULL DEFAULT 'purpose',
    sort_order      INT NOT NULL DEFAULT 0,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_prompt_categories_dimension
        CHECK (dimension IN ('purpose', 'style', 'subject', 'model', 'size'))
);

CREATE TABLE IF NOT EXISTS prompts (
    id                       BIGSERIAL PRIMARY KEY,
    status                   VARCHAR(24) NOT NULL DEFAULT 'draft',
    brand_type               VARCHAR(24) NOT NULL DEFAULT 'curated',
    provenance_type          VARCHAR(24) NOT NULL DEFAULT 'internal',
    authorization_status     VARCHAR(24) NOT NULL DEFAULT 'unknown',
    source_evidence_verified BOOLEAN NOT NULL DEFAULT FALSE,
    title_zh                 VARCHAR(240) NOT NULL,
    description_zh           TEXT NOT NULL DEFAULT '',
    purpose                  VARCHAR(96) NOT NULL DEFAULT '',
    style                    VARCHAR(96) NOT NULL DEFAULT '',
    subject                  VARCHAR(96) NOT NULL DEFAULT '',
    featured                 BOOLEAN NOT NULL DEFAULT FALSE,
    current_version          INT NOT NULL DEFAULT 1,
    published_version        INT,
    public_attribution_note  TEXT,
    use_count                BIGINT NOT NULL DEFAULT 0,
    favorite_count           BIGINT NOT NULL DEFAULT 0,
    created_by               BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_by               BIGINT REFERENCES users(id) ON DELETE SET NULL,
    published_by             BIGINT REFERENCES users(id) ON DELETE SET NULL,
    published_at             TIMESTAMPTZ,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_prompts_status
        CHECK (status IN ('draft', 'pending_review', 'published', 'offline')),
    CONSTRAINT chk_prompts_brand_type
        CHECK (brand_type IN ('original', 'authorized', 'curated', 'community')),
    CONSTRAINT chk_prompts_provenance_type
        CHECK (provenance_type IN ('internal', 'external', 'community')),
    CONSTRAINT chk_prompts_authorization_status
        CHECK (authorization_status IN ('unknown', 'original', 'authorized', 'curated', 'community', 'rejected')),
    CONSTRAINT chk_prompts_original_evidence
        CHECK (
            brand_type <> 'original'
            OR (
                source_evidence_verified = TRUE
                AND authorization_status = 'original'
            )
        ),
    CONSTRAINT chk_prompts_versions
        CHECK (
            current_version > 0
            AND (published_version IS NULL OR published_version > 0)
        )
);

CREATE INDEX IF NOT EXISTS idx_prompts_public_list
    ON prompts(status, featured DESC, published_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_prompts_taxonomy
    ON prompts(purpose, style, subject);

CREATE TABLE IF NOT EXISTS prompt_versions (
    id                  BIGSERIAL PRIMARY KEY,
    prompt_id           BIGINT NOT NULL REFERENCES prompts(id) ON DELETE CASCADE,
    version             INT NOT NULL,
    brand_type          VARCHAR(24) NOT NULL DEFAULT 'curated',
    provenance_type     VARCHAR(24) NOT NULL DEFAULT 'internal',
    authorization_status VARCHAR(24) NOT NULL DEFAULT 'unknown',
    source_evidence_verified BOOLEAN NOT NULL DEFAULT FALSE,
    title_zh            VARCHAR(240) NOT NULL,
    description_zh      TEXT NOT NULL DEFAULT '',
    purpose             VARCHAR(96) NOT NULL DEFAULT '',
    style               VARCHAR(96) NOT NULL DEFAULT '',
    subject             VARCHAR(96) NOT NULL DEFAULT '',
    featured            BOOLEAN NOT NULL DEFAULT FALSE,
    prompt_text         TEXT NOT NULL,
    variables           JSONB NOT NULL DEFAULT '{}'::jsonb,
    models              TEXT[] NOT NULL DEFAULT '{}'::text[],
    sizes               TEXT[] NOT NULL DEFAULT '{}'::text[],
    reference_requirement VARCHAR(16) NOT NULL DEFAULT 'none',
    reference_instructions TEXT NOT NULL DEFAULT '',
    requires_reference  BOOLEAN NOT NULL DEFAULT FALSE,
    public_attribution_note TEXT,
    change_note         TEXT NOT NULL DEFAULT '',
    created_by          BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_prompt_versions_prompt_version UNIQUE (prompt_id, version),
    CONSTRAINT chk_prompt_versions_version CHECK (version > 0),
    CONSTRAINT chk_prompt_versions_brand_type
        CHECK (brand_type IN ('original', 'authorized', 'curated', 'community')),
    CONSTRAINT chk_prompt_versions_provenance_type
        CHECK (provenance_type IN ('internal', 'external', 'community')),
    CONSTRAINT chk_prompt_versions_authorization_status
        CHECK (authorization_status IN ('unknown', 'original', 'authorized', 'curated', 'community', 'rejected')),
    CONSTRAINT chk_prompt_versions_reference_requirement
        CHECK (reference_requirement IN ('none', 'optional', 'required')),
    CONSTRAINT chk_prompt_versions_prompt_text CHECK (BTRIM(prompt_text) <> '')
);

CREATE TABLE IF NOT EXISTS prompt_category_links (
    prompt_id    BIGINT NOT NULL REFERENCES prompts(id) ON DELETE RESTRICT,
    version      INT NOT NULL,
    category_id  BIGINT NOT NULL REFERENCES prompt_categories(id) ON DELETE RESTRICT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (prompt_id, version, category_id),
    CONSTRAINT fk_prompt_category_links_version
        FOREIGN KEY (prompt_id, version)
        REFERENCES prompt_versions(prompt_id, version)
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_prompt_category_links_category
    ON prompt_category_links(category_id, prompt_id, version);

CREATE TABLE IF NOT EXISTS prompt_media (
    id          BIGSERIAL PRIMARY KEY,
    prompt_id   BIGINT NOT NULL REFERENCES prompts(id) ON DELETE RESTRICT,
    version     INT NOT NULL,
    media_type  VARCHAR(24) NOT NULL DEFAULT 'image',
    url         TEXT NOT NULL,
    alt_zh      TEXT NOT NULL DEFAULT '',
    sort_order  INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_prompt_media_type CHECK (media_type IN ('image', 'video', 'reference')),
    CONSTRAINT fk_prompt_media_version
        FOREIGN KEY (prompt_id, version)
        REFERENCES prompt_versions(prompt_id, version)
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_prompt_media_prompt
    ON prompt_media(prompt_id, version, sort_order, id);

CREATE TABLE IF NOT EXISTS prompt_sources (
    id                    BIGSERIAL PRIMARY KEY,
    prompt_id             BIGINT NOT NULL REFERENCES prompts(id) ON DELETE RESTRICT,
    version               INT NOT NULL,
    source_key            VARCHAR(128) NOT NULL,
    external_id           VARCHAR(256),
    source_url            TEXT,
    original_author       TEXT,
    source_payload        JSONB NOT NULL DEFAULT '{}'::jsonb,
    evidence              JSONB NOT NULL DEFAULT '{}'::jsonb,
    authorization_status  VARCHAR(24) NOT NULL DEFAULT 'unknown',
    evidence_verified     BOOLEAN NOT NULL DEFAULT FALSE,
    recorded_by           BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_prompt_sources_version
        FOREIGN KEY (prompt_id, version)
        REFERENCES prompt_versions(prompt_id, version)
        ON DELETE RESTRICT,
    CONSTRAINT chk_prompt_sources_authorization
        CHECK (authorization_status IN ('unknown', 'original', 'authorized', 'curated', 'community', 'rejected'))
);

CREATE INDEX IF NOT EXISTS idx_prompt_sources_prompt
    ON prompt_sources(prompt_id, created_at DESC);

CREATE TABLE IF NOT EXISTS prompt_favorites (
    id          BIGSERIAL PRIMARY KEY,
    prompt_id   BIGINT NOT NULL REFERENCES prompts(id) ON DELETE RESTRICT,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_prompt_favorites_prompt_user UNIQUE (prompt_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_prompt_favorites_user_created
    ON prompt_favorites(user_id, created_at DESC);

CREATE OR REPLACE FUNCTION update_prompt_favorite_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE prompts
        SET favorite_count = favorite_count + 1
        WHERE id = NEW.prompt_id;
        RETURN NEW;
    END IF;

    UPDATE prompts
    SET favorite_count = GREATEST(favorite_count - 1, 0)
    WHERE id = OLD.prompt_id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_prompt_favorite_count ON prompt_favorites;
CREATE TRIGGER trg_prompt_favorite_count
AFTER INSERT OR DELETE ON prompt_favorites
FOR EACH ROW
EXECUTE FUNCTION update_prompt_favorite_count();

CREATE TABLE IF NOT EXISTS prompt_use_events (
    id          BIGSERIAL PRIMARY KEY,
    prompt_id   BIGINT NOT NULL,
    version     INT NOT NULL,
    user_id     BIGINT REFERENCES users(id) ON DELETE SET NULL,
    metadata    JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_prompt_use_events_version
        FOREIGN KEY (prompt_id, version)
        REFERENCES prompt_versions(prompt_id, version)
        ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_prompt_use_events_prompt_created
    ON prompt_use_events(prompt_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_prompt_use_events_user_created
    ON prompt_use_events(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS prompt_import_jobs (
    id            BIGSERIAL PRIMARY KEY,
    source_key    VARCHAR(128) NOT NULL,
    status        VARCHAR(24) NOT NULL DEFAULT 'pending_review',
    raw_payload   JSONB NOT NULL DEFAULT '{}'::jsonb,
    item_count    INT NOT NULL DEFAULT 0,
    created_by    BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_prompt_import_jobs_status
        CHECK (status IN ('pending_review', 'completed', 'failed'))
);

CREATE TABLE IF NOT EXISTS prompt_import_items (
    id                    BIGSERIAL PRIMARY KEY,
    job_id                BIGINT NOT NULL REFERENCES prompt_import_jobs(id) ON DELETE RESTRICT,
    source_key            VARCHAR(128) NOT NULL,
    external_id           VARCHAR(256) NOT NULL,
    normalized_hash       VARCHAR(128) NOT NULL,
    status                VARCHAR(24) NOT NULL DEFAULT 'pending_review',
    normalized_payload    JSONB NOT NULL,
    source_payload        JSONB NOT NULL DEFAULT '{}'::jsonb,
    evidence              JSONB NOT NULL DEFAULT '{}'::jsonb,
    authorization_status  VARCHAR(24) NOT NULL DEFAULT 'unknown',
    prompt_id             BIGINT REFERENCES prompts(id) ON DELETE RESTRICT,
    reviewed_by           BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at           TIMESTAMPTZ,
    rejection_reason      TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_prompt_import_items_source_external UNIQUE (source_key, external_id),
    CONSTRAINT uq_prompt_import_items_normalized_hash UNIQUE (normalized_hash),
    CONSTRAINT chk_prompt_import_items_status
        CHECK (status IN ('pending_review', 'approved', 'rejected', 'duplicate')),
    CONSTRAINT chk_prompt_import_items_authorization
        CHECK (authorization_status IN ('unknown', 'original', 'authorized', 'curated', 'community', 'rejected'))
);

CREATE INDEX IF NOT EXISTS idx_prompt_import_items_job_status
    ON prompt_import_items(job_id, status, id);

CREATE TABLE IF NOT EXISTS prompt_review_records (
    id          BIGSERIAL PRIMARY KEY,
    prompt_id   BIGINT NOT NULL,
    version     INT NOT NULL,
    decision    VARCHAR(16) NOT NULL,
    note        TEXT NOT NULL DEFAULT '',
    reviewer_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_prompt_review_records_version
        FOREIGN KEY (prompt_id, version)
        REFERENCES prompt_versions(prompt_id, version)
        ON DELETE RESTRICT,
    CONSTRAINT chk_prompt_review_records_decision
        CHECK (decision IN ('approve', 'reject'))
);

CREATE INDEX IF NOT EXISTS idx_prompt_review_records_prompt_version
    ON prompt_review_records(prompt_id, version, created_at DESC);

CREATE TABLE IF NOT EXISTS prompt_reports (
    id             BIGSERIAL PRIMARY KEY,
    prompt_id      BIGINT NOT NULL REFERENCES prompts(id) ON DELETE RESTRICT,
    reporter_id    BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reason         VARCHAR(96) NOT NULL,
    detail         TEXT NOT NULL DEFAULT '',
    status         VARCHAR(24) NOT NULL DEFAULT 'open',
    resolution     TEXT NOT NULL DEFAULT '',
    resolved_by    BIGINT REFERENCES users(id) ON DELETE SET NULL,
    resolved_at    TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_prompt_reports_status CHECK (status IN ('open', 'resolved', 'dismissed'))
);

CREATE INDEX IF NOT EXISTS idx_prompt_reports_status_created
    ON prompt_reports(status, created_at DESC);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_prompts_current_version'
          AND conrelid = 'prompts'::regclass
    ) THEN
        ALTER TABLE prompts
            ADD CONSTRAINT fk_prompts_current_version
            FOREIGN KEY (id, current_version)
            REFERENCES prompt_versions(prompt_id, version)
            DEFERRABLE INITIALLY DEFERRED;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_prompts_published_version'
          AND conrelid = 'prompts'::regclass
    ) THEN
        ALTER TABLE prompts
            ADD CONSTRAINT fk_prompts_published_version
            FOREIGN KEY (id, published_version)
            REFERENCES prompt_versions(prompt_id, version)
            DEFERRABLE INITIALLY DEFERRED;
    END IF;
END
$$;

CREATE OR REPLACE FUNCTION validate_prompt_original_evidence()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.brand_type <> 'original' THEN
        RETURN NEW;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM prompt_sources source
        WHERE source.prompt_id = NEW.id
          AND source.version = NEW.current_version
          AND source.authorization_status = 'original'
          AND source.evidence_verified = TRUE
          AND BTRIM(source.source_key) <> ''
          AND BTRIM(COALESCE(source.external_id, '')) <> ''
          AND BTRIM(COALESCE(source.original_author, '')) <> ''
          AND BTRIM(COALESCE(source.evidence->>'summary', '')) <> ''
          AND BTRIM(COALESCE(source.evidence->>'captured_at', '')) <> ''
          AND BTRIM(COALESCE(source.evidence->>'proof_type', '')) <> ''
    ) THEN
        RAISE EXCEPTION 'original prompt requires verified structured evidence'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'chk_prompts_original_structured_evidence';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_prompts_original_evidence ON prompts;
CREATE CONSTRAINT TRIGGER trg_prompts_original_evidence
AFTER INSERT OR UPDATE OF brand_type, current_version ON prompts
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION validate_prompt_original_evidence();

ALTER TABLE image_studio_jobs
    ADD COLUMN IF NOT EXISTS prompt_id BIGINT;

ALTER TABLE image_studio_jobs
    ADD COLUMN IF NOT EXISTS prompt_version INT;

ALTER TABLE image_studio_jobs
    ADD COLUMN IF NOT EXISTS model TEXT;

ALTER TABLE image_studio_jobs
    ADD COLUMN IF NOT EXISTS quality TEXT;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_image_studio_jobs_prompt_version'
          AND conrelid = 'image_studio_jobs'::regclass
    ) THEN
        ALTER TABLE image_studio_jobs
            ADD CONSTRAINT fk_image_studio_jobs_prompt_version
            FOREIGN KEY (prompt_id, prompt_version)
            REFERENCES prompt_versions(prompt_id, version)
            ON DELETE SET NULL;
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_image_studio_jobs_prompt_pair'
          AND conrelid = 'image_studio_jobs'::regclass
    ) THEN
        ALTER TABLE image_studio_jobs
            ADD CONSTRAINT chk_image_studio_jobs_prompt_pair
            CHECK (
                (prompt_id IS NULL AND prompt_version IS NULL)
                OR (prompt_id IS NOT NULL AND prompt_version IS NOT NULL)
            );
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_image_studio_jobs_prompt_version
    ON image_studio_jobs(prompt_id, prompt_version)
    WHERE prompt_id IS NOT NULL;
