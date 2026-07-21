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
