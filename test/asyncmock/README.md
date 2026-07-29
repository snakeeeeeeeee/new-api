# Async worker acceptance mock

Start it with the opt-in Compose profile:

```bash
docker compose -f docker-compose-dev.yml --profile async-test up -d --build
```

The service is exposed on `http://localhost:18081` and provides:

- `POST /webhook/success[/name]`: immediate HTTP 204.
- `POST /webhook/failure[/name]`: immediate HTTP 500.
- `POST /webhook/delay[/name]?delay_ms=2000&status=204`: delayed response.
- `POST /v1/image/tasks`: image-handle-compatible submit response.
- `POST /v1/videos`: AdobeVideo-compatible asynchronous submit response.
- `GET /v1/videos/{id}`: configurable `queued`/`in_progress`/terminal lifecycle.
- `GET /v1/videos/{id}/content`: mock MP4 content with `Range: bytes=0-3` support.
- `GET /metrics`: current and peak image/Webhook concurrency, including per-path Webhook peaks.
- `POST /reset`: reset counters while preserving currently active requests.
- `GET|PUT /control`: inspect or update default image/Webhook status and delay.

Example image failure control:

```bash
curl -X PUT http://localhost:18081/control \
  -H 'Content-Type: application/json' \
  -d '{"image_status":503,"image_delay_ms":0}'
```

An image request can override the defaults with numeric metadata fields named
`async_test_status` and `async_test_delay_ms`.

Video defaults can be changed through `/control` with
`video_submit_status`, `video_terminal_status` (`completed` or `failed`), and
`video_terminal_after_poll`. `/metrics` exposes submit/poll/content counts and
the last normalized video payload.
