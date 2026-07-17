# Asynchronous Image Tasks

Asynchronous image tasks let clients submit long-running OpenAI-compatible image requests without keeping one HTTP connection open. This avoids proxy/CDN response timeouts such as Cloudflare 524 while preserving the existing image routing, billing, moderation, concurrency, and failover behavior.

## Endpoints

The authenticated gateway exposes both `/v1` paths and their existing no-prefix aliases:

```text
POST /v1/images/generations/async
POST /v1/images/edits/async
GET  /v1/images/tasks/{task_id}
```

The aliases are `/images/generations/async`, `/images/edits/async`, and `/images/tasks/{task_id}`.

Only OpenAI and Grok groups are supported. Requests use the same JSON or multipart payload as the corresponding synchronous endpoint. Streaming image requests are rejected because a polled task returns one final JSON result.

## Enabling the feature (result storage)

Asynchronous image tasks are **disabled by default** and gated on result storage. When the switch is off, the async endpoints return `404` and never create a task or write to Redis. This is deliberate: without offloading, large `b64_json` results (several MB each, e.g. `gpt-image-1`) would accumulate in Redis and exhaust its memory.

For single-instance Zeabur/Docker deployments, use the local persistent data volume:

```yaml
image_storage:
  enabled: true
  backend: "local"
  local_dir: ""                       # empty -> DATA_DIR/image-task-results
  local_url_prefix: "/v1/images/task-assets/"
  prefix: "images/"
  max_download_bytes: 33554432
```

Equivalent environment variables:

```text
IMAGE_STORAGE_ENABLED=true
IMAGE_STORAGE_BACKEND=local
IMAGE_STORAGE_LOCAL_DIR=
IMAGE_STORAGE_LOCAL_URL_PREFIX=/v1/images/task-assets/
IMAGE_STORAGE_PREFIX=images/
IMAGE_STORAGE_MAX_DOWNLOAD_BYTES=33554432
```

With `backend: local`, completed images are stored under the instance data volume and returned as authenticated `/v1/images/task-assets/...` URLs. The same API key that submitted the task must be used to download the asset.

For multi-replica deployments or CDN distribution, configure an S3-compatible object store (AWS S3, Cloudflare R2, Aliyun OSS, MinIO, …):

```yaml
image_storage:
  enabled: true
  backend: "s3"
  endpoint: "https://<account_id>.r2.cloudflarestorage.com"  # AWS official S3 can leave empty
  region: "auto"
  bucket: "my-images"
  access_key_id: "..."
  secret_access_key: "..."
  prefix: "images/"
  force_path_style: false          # MinIO/path-style buckets set true
  public_base_url: ""              # set to return public_base_url/key直链; empty → presigned URL
  presign_expiry_hours: 24         # presigned link TTL when public_base_url is empty
  max_download_bytes: 33554432     # cap when re-hosting an upstream image URL (32MB)
```

S3 environment variables:

```text
IMAGE_STORAGE_ENABLED=true
IMAGE_STORAGE_BACKEND=s3
IMAGE_STORAGE_ENDPOINT=https://<account_id>.r2.cloudflarestorage.com
IMAGE_STORAGE_REGION=auto
IMAGE_STORAGE_BUCKET=my-images
IMAGE_STORAGE_ACCESS_KEY_ID=...
IMAGE_STORAGE_SECRET_ACCESS_KEY=...
IMAGE_STORAGE_PREFIX=images/
IMAGE_STORAGE_FORCE_PATH_STYLE=false
IMAGE_STORAGE_PUBLIC_BASE_URL=
IMAGE_STORAGE_PRESIGN_EXPIRY_HOURS=24
IMAGE_STORAGE_MAX_DOWNLOAD_BYTES=33554432
```

Production readiness check:

```bash
curl -i https://api.jisudeng.com/v1/images/generations/async \
  -H 'Authorization: Bearer sk-...' \
  -H 'Content-Type: application/json' \
  -d '{"model":"gpt-image-2","prompt":"async smoke test","size":"1024x1024","n":1}'
```

The expected submit response is `202 Accepted` with `task_id` and `poll_url`. A `404` body with `async image tasks are not enabled` means `IMAGE_STORAGE_ENABLED` is not true, or `IMAGE_STORAGE_BACKEND=s3` was selected without complete bucket/access key/secret credentials.

Use the async endpoints for production image requests that may take longer than 60-90 seconds. Synchronous `/v1/images/generations` calls can still finish upstream after a CDN/Cloudflare `524`, which means the dashboard may show a successful generated image while the caller only received a timeout. Retrying the synchronous request can create duplicate paid generations.

When a task completes, each generated image is stored through the selected backend and the result is rewritten to a compact form: `data[].url` points at the stored image and `b64_json` is removed. Only this small JSON is stored in Redis. If storage fails, the task is marked `failed` rather than persisting the raw base64.

To support a different storage backend, implement the `service.ImageStorage` interface (`Save(ctx, key, contentType, data) (url, error)`) and provide it in place of the built-in local/S3 implementations.

## Submit a task

```bash
curl -i https://api.jisudeng.com/v1/images/generations/async \
  -H 'Authorization: Bearer sk-...' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gpt-image-1",
    "prompt": "A lighthouse during a winter storm",
    "size": "1536x1024"
  }'
```

The server stores the initial task in Redis and responds with `202 Accepted`:

```json
{
  "id": "imgtask_0123456789abcdef",
  "task_id": "imgtask_0123456789abcdef",
  "object": "image.generation.task",
  "status": "processing",
  "created_at": 1784092800,
  "expires_at": 1784179200,
  "poll_url": "/v1/images/tasks/imgtask_0123456789abcdef"
}
```

`Location` contains the polling path and `Retry-After: 3` provides the recommended polling interval.

## Poll a task

Use the same API key that submitted the task:

```bash
curl https://api.jisudeng.com/v1/images/tasks/imgtask_0123456789abcdef \
  -H 'Authorization: Bearer sk-...'
```

While work is in progress:

```json
{
  "id": "imgtask_0123456789abcdef",
  "task_id": "imgtask_0123456789abcdef",
  "object": "image.generation.task",
  "status": "processing",
  "created_at": 1784092800,
  "expires_at": 1784179200
}
```

On success, `result` mirrors the synchronous image API body, except each image has been offloaded to result storage: `data[].url` points at the stored image and `b64_json` is stripped (so both URL and base64 upstream formats end up as compact stored links):

```json
{
  "id": "imgtask_0123456789abcdef",
  "task_id": "imgtask_0123456789abcdef",
  "object": "image.generation.task",
  "status": "completed",
  "http_status": 200,
  "image_url": "https://...",
  "result": {
    "created": 1784092923,
    "data": [{"url": "https://..."}]
  },
  "created_at": 1784092800,
  "completed_at": 1784092923,
  "expires_at": 1784179323
}
```

For URL responses, `image_url` mirrors the first `data[].url` for simple clients. On failure, the task reaches `failed` and exposes the original OpenAI-compatible error object where available:

```json
{
  "id": "imgtask_0123456789abcdef",
  "task_id": "imgtask_0123456789abcdef",
  "object": "image.generation.task",
  "status": "failed",
  "http_status": 502,
  "error": {
    "type": "api_error",
    "message": "Upstream request failed"
  },
  "created_at": 1784092800,
  "completed_at": 1784092923,
  "expires_at": 1784179323
}
```

Successful submit and poll responses include `Cache-Control: no-store`, preventing a CDN from caching the `processing` state. Tasks and results expire 24 hours after their latest state update. A task executes for at most 30 minutes.

Task ownership is scoped to both user and API key. Unknown task IDs and IDs owned by another key both return `404`, avoiding task-existence disclosure. Polling remains available when the completed generation used the key's remaining balance; normal authentication, disabled-key, user, IP, and group checks still apply.
