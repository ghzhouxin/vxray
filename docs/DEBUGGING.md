# Local Brew Services Debugging

## 目标

- 不影响已安装的正式版 `ghzhouxin/vxray/vxray`
- 使用独立的 `~/.vxray-dev`
- 用单独端口验证本地改动

## 当前约定

- 正式版 service：
  - label: `homebrew.mxcl.vxray`
  - port: `11888`
  - home：`~/.vxray-de`
- 调试版 service：
  - port: `10888`
  - home：`~/.vxray-dev`

## 前置条件

先确保：

- 已安装正式版 Homebrew formula：`ghzhouxin/vxray/vxray`
- 本机已安装 `xray`

本地辅助调试脚本：`scripts/dev.sh`

## 本地调试

```bash
scripts/dev.sh start
scripts/dev.sh check # 检查vxray服务状态
scripts/dev.sh logs # 查看vxray服务日志
```

说明：

- 运行本地编译后的后端二进制
- 调试 service 会注入：
  - `VXRAY_HOME=~/.vxray-dev`
  - `VXRAY_WEB_ROOT=web/dist`
  - `VXRAY_XRAY_BINARY=/opt/homebrew/bin/xray`
  - `VXRAY_SERVER_PORT=10888`
