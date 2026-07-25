# API Reference

Base URL: `http://127.0.0.1:10888/api`

---

## Subscriptions

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/subscriptions` | List all subscriptions |
| POST | `/subscriptions` | Create subscription |
| PUT | `/subscriptions/:id` | Update subscription |
| DELETE | `/subscriptions/:id` | Delete subscription |
| POST | `/subscriptions/refresh` | Refresh subscriptions by ids or all |

---

## Nodes

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/nodes` | List nodes (supports filtering) |
| GET | `/nodes/speedtest/status` | Get speed test task status |
| POST | `/nodes/speedtest` | Batch speed test (SSE stream) |
| POST | `/nodes/:id/activate` | Activate node |
| DELETE | `/nodes/:id` | Delete node |
| POST | `/nodes/delete-failed` | Delete failed nodes |

### Query Parameters for GET /nodes

| Parameter | Description |
|-----------|-------------|
| `subscription_id` | Filter by subscription |
| `protocol` | Filter by protocol |
| `keyword` | Filter by name keyword |

---

## Xray

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/console` | Get console snapshot |
| GET | `/xray/runtime` | Get running status |
| POST | `/xray/runtime/start` | Start Xray |
| POST | `/xray/runtime/stop` | Stop Xray |
| POST | `/xray/runtime/restart-best` | Restart with best tested node |
| GET | `/xray/config` | Get Xray config |
| GET | `/xray/config/default` | Get default Xray config |
| PUT | `/xray/config` | Save Xray config |
| POST | `/xray/speedtest/websites` | Website speed test via Xray proxy |

---

## Proxy (macOS only)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/proxy/toggle` | Toggle system proxy |

---

## Geo Files

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/geo/status` | Get geo files status |
| POST | `/geo/download/all` | Download all geo files |

---

## Settings

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/settings` | Get current settings |
| GET | `/settings/default` | Get default settings |
| PUT | `/settings` | Update settings |

---

## Logs

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/logs` | List logs |
| DELETE | `/logs` | Clear all logs |

---

## Clash

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/clash` | Generate Clash subscription config (YAML) |

---

## Error Response

```json
{
  "code": 400,
  "message": "error description"
}
```
