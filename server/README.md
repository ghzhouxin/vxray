# VXRay Web Manager - 后端服务

基于 Go 语言实现的 V2Ray/Xray Web 管理工具后端服务。

## 技术栈

- Go 1.21+
- Gin Web Framework
- GORM (SQLite 数据库)
- Xray-core

## 项目结构

```
server/
├── cmd/main.go              # 入口程序
├── internal/
│   ├── api/                 # HTTP API 处理层
│   │   ├── routes.go        # 路由注册
│   │   ├── node.go          # 节点 API
│   │   ├── subscription.go  # 订阅 API
│   │   ├── xray.go          # Xray 控制 API
│   │   ├── config.go        # 配置 API
│   │   ├── geo.go           # Geo 文件 API
│   │   ├── log.go           # 日志 API
│   │   └── proxy.go         # 系统代理 API
│   ├── config/              # 配置管理
│   ├── constants/           # 常量定义
│   ├── dto/                 # 数据传输对象
│   ├── model/               # 数据模型
│   └── service/             # 业务逻辑层
├── pkg/
│   ├── geo/                 # Geo 文件管理
│   ├── response/            # 统一响应封装
│   ├── speedtest/           # 测速实现
│   ├── subscription/        # 订阅解析服务
│   ├── types/               # 公共类型定义
│   └── utils/               # 工具函数
├── go.mod
└── go.sum
```

## 功能特性

### 1. 订阅管理
- 添加、删除、更新订阅
- 自动解析订阅链接
- 支持多种协议：vless, vmess, trojan, shadowsocks, socks, hysteria2

### 2. 节点管理
- 自动解析节点信息
- 单节点测速
- 批量并发测速
- SSE 实时推送测速进度

### 3. Xray 进程管理
- 启动/停止 Xray 进程
- 自动生成配置文件
- 进程状态监控

### 4. 系统代理
- macOS 系统代理一键设置
- 支持 HTTP/SOCKS5 代理

### 5. Geo 文件管理
- 自动下载 Geo 文件
- 文件状态检查

## 快速开始

### 1. 安装依赖

```bash
cd server
go mod tidy
```

### 2. 构建并运行

```bash
go build -o vxray-server ./cmd
./vxray-server
```

服务将在 `http://127.0.0.1:8080` 启动。

## API 接口

### 订阅管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/subscriptions | 获取订阅列表 |
| POST | /api/subscriptions | 创建订阅 |
| PUT | /api/subscriptions/:id | 更新订阅 |
| DELETE | /api/subscriptions/:id | 删除订阅 |
| POST | /api/subscriptions/refresh | 刷新订阅节点 |

### 节点管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/nodes | 获取节点列表 |
| GET | /api/nodes/speedtest/status | 获取测速任务状态 |
| POST | /api/nodes/speedtest | 按筛选条件流式测速 |
| POST | /api/nodes/:id/activate | 激活节点 |
| DELETE | /api/nodes/:id | 删除节点 |
| POST | /api/nodes/delete-failed | 删除超时节点 |

### Xray 管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/console | 获取 Console 首页快照 |
| GET | /api/xray/runtime | 获取运行状态 |
| POST | /api/xray/runtime/start | 启动 Xray |
| POST | /api/xray/runtime/stop | 停止 Xray |
| GET | /api/xray/config | 获取 Xray 配置 |
| GET | /api/xray/config/default | 获取默认 Xray 配置 |
| PUT | /api/xray/config | 保存 Xray 配置 |
| POST | /api/xray/speedtest/websites | 网站测速 |

### 系统代理

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/proxy/toggle | 切换系统代理 |

### Geo 文件

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/geo/status | 获取 Geo 文件状态 |
| POST | /api/geo/download/all | 下载所有 Geo 文件 |

### 日志管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/logs | 获取日志列表 |
| DELETE | /api/logs | 清空数据库日志数据 |

### 设置管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/settings | 获取设置 |
| GET | /api/settings/default | 获取默认设置 |
| PUT | /api/settings | 更新设置 |

## 数据模型

```go
type Subscription struct {
    ID        uint      `json:"id" gorm:"primaryKey"`
    Name      string    `json:"name"`
    URL       string    `json:"url"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

type Node struct {
    ID             uint                   `json:"id" gorm:"primaryKey"`
    SubscriptionID uint                   `json:"subscription_id"`
    Name           string                 `json:"name"`
    Protocol       string                 `json:"protocol"`
    Address        string                 `json:"address"`
    Port           int                    `json:"port"`
    RawConfig      map[string]interface{} `json:"raw_config"`
    OutboundConfig map[string]interface{} `json:"outbound_config"`
    Latency        int64                  `json:"latency"`
    CreatedAt      time.Time              `json:"created_at"`
    UpdatedAt      time.Time              `json:"updated_at"`
}
```

## 支持的协议

- VLESS (支持 Reality, TLS, WebSocket, gRPC)
- VMess (支持 TLS, WebSocket, gRPC, HTTPUpgrade)
- Trojan (支持 TLS, WebSocket, gRPC)
- Shadowsocks
- SOCKS

## 目录结构

```
~/.vxray/                      # 应用主目录
├── data/vxray.db              # SQLite 数据库（含配置、订阅、节点、日志）
├── geo/                       # Geo 文件目录
│   ├── geoip.dat
│   └── geosite.dat
└── xray/config.json           # Xray 配置文件
```

## License

MIT
