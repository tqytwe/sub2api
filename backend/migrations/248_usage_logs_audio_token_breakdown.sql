-- Preserve the billable audio subsets emitted by OpenAI Realtime response.done
-- events. The existing total token columns remain canonical for historical
-- aggregates, while these columns make the audio pricing components auditable.
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS input_audio_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS output_audio_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cache_creation_audio_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cache_read_audio_tokens INTEGER NOT NULL DEFAULT 0;
