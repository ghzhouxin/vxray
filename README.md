# VXRay

Local Xray manager with web UI.

## Installation

```bash
brew tap ghzhouxin/vxray
brew install ghzhouxin/vxray/vxray
brew services start ghzhouxin/vxray/vxray
```

Access at http://127.0.0.1:11888/·

## Development

### Local debugging

本地调试使用：

- port: `10888`
- data dir: `~/.vxray-dev`

调试：

```bash
scripts/dev.sh start
scripts/dev.sh check # 检查vxray服务状态
scripts/dev.sh logs # 查看vxray服务日志
```

See [docs/debugging](docs/DEBUGGING.md) for debugging details.

## Documentation

See [docs/README.md](./docs/README.md) for details.
