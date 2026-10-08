-- Permanent submission tombstones; deliberately no TTL or cascading deletion.
CREATE TABLE IF NOT EXISTS starframe_video_submissions (
 local_id TEXT PRIMARY KEY,
 user_id BIGINT NOT NULL,
 api_key_id BIGINT NOT NULL,
 group_id BIGINT NOT NULL,
 client_task_id TEXT NOT NULL,
 task_json TEXT NOT NULL,
 completed BOOLEAN NOT NULL DEFAULT FALSE,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE (user_id, api_key_id, group_id, client_task_id)
);
