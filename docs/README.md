# VXRay Documentation

## Architecture

- Single binary deployment with single backend service serving bundled static web assets
- Backend serves both API (`/api/**`) and static files
- Data directory: `~/.vxray`
- Shared frontend state follows `view -> store -> api`; single-page-only flows may call API directly
- `pkg` remains stateless; business state lives in `internal/service`
- Subscription parsing remains protocol-specific; common helpers only cover shared skeletons

---

## Configuration

Config file: `~/.vxray/config.yaml`

```yaml
server:
  host: 127.0.0.1
  port: 11888

log:
  level: info

xray:
  binary: xray

speedtest:
  target_url: https://www.google.com/generate_204
  timeout: 2000
  concurrency: 20
```

### Config Options

| Option                    | Default       | Description            |
| ------------------------- | ------------- | ---------------------- |
| `server.host`           | `127.0.0.1` | Listen address         |
| `server.port`           | `11888`     | API port               |
| `log.level`             | `info`      | Log level              |
| `xray.binary`           | `xray`      | Xray binary path       |
| `speedtest.timeout`     | `2000`      | Timeout in ms          |
| `speedtest.concurrency` | `20`        | Concurrent connections |

---

## Directory Structure

```
~/.vxray/
├── config.yaml      # Main config
├── data/
│   └── vxray.db     # SQLite database
├── geo/
│   ├── geoip.dat
│   └── geosite.dat
└── xray/
    └── config.json  # Xray config
```

---

## Xray Inbound Ports

| Protocol | Port      |
| -------- | --------- |
| SOCKS5   | `18888` |
| HTTP     | `18889` |

---

## Environment Variables

| Variable              | Description                |
| --------------------- | -------------------------- |
| `VXRAY_HOME`        | Data directory             |
| `VXRAY_WEB_ROOT`    | Web static files directory |
| `VXRAY_XRAY_BINARY` | Xray binary path           |

---

## API Reference

See [API.md](./API.md) for complete API documentation.
