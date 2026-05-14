# EasyNode — 智能代理节点控制面板 开发指导文档

## 项目概述

EasyNode 是一个面向普通用户的智能代理节点管理面板。核心理念是**"零配置、全自动、美观、安全"**——用户只需输入域名，系统自动完成所有协议配置，以卡片式极简 UI 呈现可用节点。

**一句话定位**：把专业的事交给系统，把简单的体验留给用户。

---

## 目标用户

- 拥有一台 VPS 和一个域名，想快速搭建代理节点的普通用户
- 不想（也不需要）理解 VLESS、Reality、WebSocket 等底层概念
- 希望界面美观、操作简单、节点稳定

---

## 技术选型

| 组件 | 选择 | 理由 |
|------|------|------|
| **代理内核** | sing-box | 统一内核，支持所有主流协议（VLESS/Trojan/Hysteria2/TUIC），活跃维护，性能优秀 |
| **后端语言** | Go | 编译为单二进制文件，零运行时依赖，性能好，与 sing-box 同生态 |
| **Web 框架** | Gin 或 Fiber | 轻量高性能的 Go HTTP 框架 |
| **前端框架** | React 18+ + TypeScript | 现代前端标准，生态丰富 |
| **UI 组件库** | shadcn/ui + Tailwind CSS | 现代美观，高度可定制，适合卡片式设计 |
| **动画** | Framer Motion | 流畅的过渡动画和交互反馈 |
| **图表** | recharts（轻量）或 sparkline 自绘 | 流量统计可视化 |
| **数据库** | SQLite (embedded, via go-sqlite3 或 modernc.org/sqlite) | 零配置，单文件，适合单机场景 |
| **证书管理** | 内置 ACME 客户端 (golang.org/x/crypto/acme) | 自动申请 + 自动续期 Let's Encrypt 证书 |
| **前端嵌入** | Go embed.FS | 前端编译为静态文件嵌入 Go 二进制，最终交付单文件 |
| **部署** | 单二进制 + install.sh 一键安装脚本 | `curl -fsSL https://xxx/install.sh | bash` |

---

## 系统架构

```
                    ┌──────────────────────────────┐
                    │         用户浏览器             │
                    │    (React SPA, 卡片式 UI)      │
                    └──────────────┬───────────────┘
                                   │ HTTPS
                    ┌──────────────▼───────────────┐
                    │       EasyNode 主进程          │
                    │  ┌─────────────────────────┐  │
                    │  │     REST API (Go)        │  │
                    │  │  · 节点 CRUD             │  │
                    │  │  · 智能推荐引擎          │  │
                    │  │  · 链式代理管理          │  │
                    │  │  · ACME 证书管理         │  │
                    │  │  · 订阅链接生成          │  │
                    │  │  · 系统监控              │  │
                    │  └────────────┬────────────┘  │
                    │               │ 进程管理/API    │
                    │  ┌────────────▼────────────┐  │
                    │  │     sing-box 子进程      │  │
                    │  │  · VLESS + Reality       │  │
                    │  │  · Trojan + TLS          │  │
                    │  │  · Hysteria2             │  │
                    │  │  · TUIC v5               │  │
                    │  └─────────────────────────┘  │
                    │  ┌─────────────────────────┐  │
                    │  │  SQLite (embedded)       │  │
                    │  └─────────────────────────┘  │
                    └──────────────┬───────────────┘
                                   │ WireGuard / sing-box outbound
                    ┌──────────────▼───────────────┐
                    │   链式代理落地节点（可选）      │
                    └──────────────────────────────┘
```

---

## 项目目录结构

```
easynode/
├── cmd/
│   └── easynode/
│       └── main.go                # 程序入口
├── internal/
│   ├── api/                       # HTTP API 路由和处理器
│   │   ├── router.go
│   │   ├── middleware/
│   │   │   ├── auth.go            # JWT 认证中间件
│   │   │   └── disguise.go        # 面板伪装中间件（非正确路径返回伪装页）
│   │   ├── handler/
│   │   │   ├── auth.go            # 登录/登出/修改密码
│   │   │   ├── setup.go           # 初始化向导
│   │   │   ├── node.go            # 节点管理
│   │   │   ├── subscribe.go       # 订阅链接
│   │   │   ├── chain.go           # 链式代理
│   │   │   ├── monitor.go         # 监控统计
│   │   │   └── system.go          # 系统设置/更新
│   │   └── dto/                   # 请求/响应数据结构
│   ├── core/                      # 核心业务逻辑
│   │   ├── singbox/               # sing-box 进程管理
│   │   │   ├── manager.go         # 启动/停止/重载 sing-box
│   │   │   ├── config.go          # 配置文件生成
│   │   │   └── template.go        # 各协议配置模板
│   │   ├── detector/              # 环境探测引擎
│   │   │   ├── detector.go        # 主探测逻辑
│   │   │   ├── network.go         # 网络环境检测（IP/端口/丢包/延迟）
│   │   │   ├── system.go          # 系统环境检测（OS/架构/资源）
│   │   │   └── dns.go             # DNS 解析验证
│   │   ├── recommender/           # 智能协议推荐引擎
│   │   │   ├── engine.go          # 推荐算法主逻辑
│   │   │   └── protocols.go       # 协议定义和评分模型
│   │   ├── cert/                  # ACME 证书管理
│   │   │   └── acme.go
│   │   ├── chain/                 # 链式代理
│   │   │   ├── pairing.go         # 配对码生成与验证
│   │   │   ├── tunnel.go          # 隧道建立与管理
│   │   │   └── routing.go         # 分流规则
│   │   └── subscribe/             # 订阅链接生成
│   │       ├── generator.go       # 通用订阅生成
│   │       ├── clash.go           # Clash 格式
│   │       ├── v2ray.go           # V2rayN Base64 格式
│   │       └── singbox.go         # sing-box 客户端格式
│   ├── model/                     # 数据模型
│   │   ├── user.go
│   │   ├── node.go
│   │   ├── traffic.go
│   │   └── chain.go
│   ├── store/                     # 数据库访问层
│   │   ├── db.go                  # SQLite 初始化和迁移
│   │   ├── user_store.go
│   │   ├── node_store.go
│   │   └── traffic_store.go
│   └── util/                      # 工具函数
│       ├── crypto.go              # 加密/UUID/密钥生成
│       ├── network.go             # 网络工具
│       └── system.go              # 系统工具
├── web/                           # 前端源码
│   ├── src/
│   │   ├── App.tsx
│   │   ├── main.tsx
│   │   ├── pages/
│   │   │   ├── Setup.tsx          # 初始化向导（2 步）
│   │   │   ├── Login.tsx          # 登录页
│   │   │   ├── Dashboard.tsx      # 主面板（节点卡片列表）
│   │   │   ├── ChainProxy.tsx     # 链式代理管理
│   │   │   ├── Monitor.tsx        # 监控统计
│   │   │   └── Settings.tsx       # 系统设置
│   │   ├── components/
│   │   │   ├── NodeCard.tsx        # 节点卡片组件（核心）
│   │   │   ├── SetupWizard.tsx     # 初始化向导步骤
│   │   │   ├── QRCodeModal.tsx     # 二维码弹窗
│   │   │   ├── TrafficChart.tsx    # 流量 sparkline
│   │   │   ├── StatusBadge.tsx     # 状态指示器
│   │   │   └── PairingDialog.tsx   # 配对码输入弹窗
│   │   ├── hooks/                  # 自定义 Hooks
│   │   ├── lib/                    # 工具函数
│   │   └── styles/                 # 全局样式
│   ├── package.json
│   ├── tailwind.config.ts
│   ├── tsconfig.json
│   └── vite.config.ts
├── scripts/
│   └── install.sh                 # 一键安装脚本
├── Makefile                       # 构建命令
├── go.mod
├── go.sum
└── README.md
```

---

## 核心功能详细规格

### 1. 初始化向导（首次访问时触发）

**仅 2 步，不能更多：**

#### Step 1 — 设置管理员账户
```
输入项:
  - 管理员密码（必填，最少 8 位）
  - 面板访问路径（可选，默认随机生成如 /panel-a8x3k）
```

#### Step 2 — 绑定域名
```
输入项:
  - 域名（如 example.com）
  - [复选框] "我没有域名，使用 IP 直连模式"

点击"开始部署"后自动执行:
  1. 验证域名 DNS 解析指向本机 IP
  2. 申请 Let's Encrypt TLS 证书
  3. 探测服务器网络环境（IP 版本、端口、丢包率、地区）
  4. 智能推荐引擎生成最优协议组合
  5. 生成 sing-box 配置并启动
  6. 展示进度动画（约 30-60 秒）
  7. 完成后跳转到 Dashboard 展示节点卡片
```

### 2. 智能协议推荐引擎

**输入**（自动探测，无需用户参与）：

| 探测项 | 探测方法 | 影响决策 |
|--------|---------|---------|
| 服务器 IP 地区 | ip-api.com 或内置 GeoIP | 决定伪装策略 |
| IPv4/IPv6 支持 | 尝试绑定 + 外部查询 | 双栈监听配置 |
| 端口可用性 | TCP/UDP 端口扫描 443, 8443 等 | 端口分配 |
| UDP 连通性 | 发送 QUIC probe | 决定是否推荐 Hysteria2/TUIC |
| 网络丢包率 | ping 测试 | 高丢包优先 QUIC 协议 |
| TLS 到目标站点 | 检测 SNI 审查 | Reality vs 传统 TLS |

**输出**（协议推荐矩阵）：

```go
type ProtocolRecommendation struct {
    Protocol    string  // "vless-reality", "hysteria2", "trojan-tls", "vless-ws-tls", "tuic"
    Transport   string  // "tcp", "ws", "quic"
    Security    string  // "reality", "tls", "none"
    Priority    int     // 1-5, 5 最高
    Label       string  // 用户可见的标签，如 "抗封锁首选"
    Description string  // 简短描述，如 "最强伪装，基于 Reality 协议"
    Enabled     bool    // 是否默认启用
}
```

**推荐策略**：

| 协议 | 条件 | 默认启用 | 标签 |
|------|------|---------|------|
| VLESS + Reality + Vision | 服务器有公网 IPv4 | ✅ 是 | 抗封锁首选 |
| Hysteria2 | UDP 端口可用且连通 | ✅ 是 | 高速传输 |
| Trojan + TLS | 有域名和证书 | ✅ 是 | 广泛兼容 |
| VLESS + WS + TLS | 有域名和证书 | ❌ 否（用户可开启） | CDN 中转 |
| TUIC v5 | UDP 可用 | ❌ 否（用户可开启） | QUIC 备选 |

### 3. 节点卡片 UI 组件

每张卡片展示以下信息：

```typescript
interface NodeCardProps {
  id: string;
  protocol: string;          // 协议名称，如 "VLESS Reality"
  label: string;             // 标签，如 "抗封锁首选"
  description: string;       // 简短描述
  priority: number;          // 推荐星级 1-5
  status: "running" | "stopped" | "error";
  latency: number | null;    // 延迟 ms
  trafficUsed: number;       // 已用流量 bytes
  trafficTotal: number | null; // 总流量限制（null 表示无限制）
  createdAt: string;

  // 操作
  onCopyLink: () => void;    // 复制节点链接
  onShowQR: () => void;      // 显示二维码
  onToggle: () => void;      // 启用/停用
  onViewDetail: () => void;  // 查看详情（专家模式）
}
```

**卡片视觉规范**：

- 圆角: 16px
- 背景: 暗色主题下 `bg-zinc-900/80` + `backdrop-blur`
- 边框: 细微发光边框，运行状态为绿色辉光，停止为灰色
- 状态指示: 左上角圆点（🟢 运行 / 🔴 错误 / ⚪ 停止）
- 推荐标签: 右上角彩色 badge
- 间距: 卡片间 16px gap，grid 布局响应式（1/2/3 列）
- 悬停: 微微上浮 + 阴影增强
- 操作按钮: 底部一行，图标 + 文字，hover 时颜色高亮

### 4. 链式代理

#### 4.1 配对码机制

```
落地节点（服务器 B）:
  1. 用户点击 "生成配对码"
  2. 系统生成 6 位字母数字配对码（如 A3X-K9M）
  3. 配对码有效期 5 分钟
  4. 配对码关联: 服务器 B 的公网 IP、一次性密钥对、端口信息

入口节点（服务器 A）:
  1. 用户点击 "添加落地节点"
  2. 输入配对码
  3. 系统向 B 发起配对请求（通过 B 的面板 API）
  4. 双方完成密钥交换
  5. A 自动在 sing-box outbound 中添加链式转发
  6. B 自动允许来自 A 的入站连接
```

#### 4.2 配对 API

```
POST /api/v1/chain/generate-code
Response: { "code": "A3X-K9M", "expires_at": "...", "port": 12345 }

POST /api/v1/chain/pair
Body: { "code": "A3X-K9M", "my_public_key": "...", "my_endpoint": "..." }
Response: { "peer_public_key": "...", "peer_endpoint": "...", "tunnel_config": {...} }
```

#### 4.3 分流规则（Phase 3）

```typescript
interface RoutingRule {
  id: string;
  name: string;           // 如 "流媒体走日本"
  exitNode: string;       // 落地节点 ID
  matchType: "domain" | "geosite" | "geoip" | "all";
  matchValue: string[];   // 如 ["netflix.com", "dmm.co.jp"] 或 ["geosite:netflix"]
  priority: number;
}
```

### 5. 订阅链接

```
GET /api/v1/subscribe/{token}
```

- `token` 为随机生成的 URL 安全字符串（32 位）
- 根据 User-Agent 或 query param 自动返回对应格式:
  - `?format=clash` → Clash YAML
  - `?format=v2ray` → V2rayN Base64
  - `?format=singbox` → sing-box JSON
  - `?format=surge` → Surge 配置
  - 默认根据 UA 自动检测

### 6. 安全设计

#### 面板伪装
```go
// middleware/disguise.go
// 所有非面板路径的请求返回伪装内容
func DisguiseMiddleware(panelPath string) gin.HandlerFunc {
    return func(c *gin.Context) {
        if !strings.HasPrefix(c.Request.URL.Path, panelPath) {
            // 返回伪装页面（默认 nginx 欢迎页或自定义 HTML）
            c.HTML(200, "disguise.html", nil)
            c.Abort()
            return
        }
        c.Next()
    }
}
```

#### 登录安全
- JWT Token 认证，有效期可配置（默认 24h）
- 连续 5 次登录失败锁定 15 分钟
- 可选 TOTP 二次验证
- 登录通知（可选 Telegram 推送）

#### 面板访问
- 默认随机高位端口（非 8080/8443 等常见端口）
- 随机面板路径（如 `/panel-a8x3k`）
- 强制 HTTPS（自签证书或 ACME 证书）

---

## 数据库 Schema (SQLite)

```sql
-- 系统设置
CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- 管理员用户
CREATE TABLE users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT NOT NULL DEFAULT 'admin',
    password_hash TEXT NOT NULL,
    totp_secret   TEXT,          -- 可选 2FA
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 代理节点
CREATE TABLE nodes (
    id          TEXT PRIMARY KEY,   -- UUID
    protocol    TEXT NOT NULL,       -- vless-reality, hysteria2, trojan-tls, etc.
    label       TEXT NOT NULL,       -- 用户可见标签
    description TEXT,
    priority    INTEGER DEFAULT 3,
    enabled     BOOLEAN DEFAULT 1,
    port        INTEGER NOT NULL,
    config_json TEXT NOT NULL,       -- sing-box inbound 配置 JSON
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 流量统计（按小时聚合）
CREATE TABLE traffic_stats (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id   TEXT NOT NULL REFERENCES nodes(id),
    upload    INTEGER NOT NULL DEFAULT 0,   -- bytes
    download  INTEGER NOT NULL DEFAULT 0,   -- bytes
    hour      DATETIME NOT NULL,            -- 精确到小时
    UNIQUE(node_id, hour)
);

-- 链式代理落地节点
CREATE TABLE chain_peers (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,       -- 用户可见名称，如 "🇯🇵 Tokyo"
    endpoint    TEXT NOT NULL,       -- 对端地址
    public_key  TEXT NOT NULL,       -- 对端公钥
    tunnel_port INTEGER,
    status      TEXT DEFAULT 'active',  -- active, inactive, error
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 分流规则
CREATE TABLE routing_rules (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    peer_id     TEXT NOT NULL REFERENCES chain_peers(id),
    match_type  TEXT NOT NULL,       -- domain, geosite, geoip, all
    match_value TEXT NOT NULL,       -- JSON array
    priority    INTEGER DEFAULT 0,
    enabled     BOOLEAN DEFAULT 1
);

-- 订阅 Token
CREATE TABLE subscribe_tokens (
    id         TEXT PRIMARY KEY,
    token      TEXT NOT NULL UNIQUE,
    name       TEXT,                 -- 备注名
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME             -- NULL 表示永不过期
);
```

---

## API 设计

### 认证

```
POST   /api/v1/auth/login          # 登录
POST   /api/v1/auth/logout         # 登出
PUT    /api/v1/auth/password       # 修改密码
```

### 初始化

```
GET    /api/v1/setup/status        # 是否已初始化
POST   /api/v1/setup/init          # 执行初始化（密码+域名）
```

### 节点

```
GET    /api/v1/nodes               # 获取所有节点（卡片列表数据）
GET    /api/v1/nodes/:id           # 获取单个节点详情
PUT    /api/v1/nodes/:id/toggle    # 启用/停用节点
GET    /api/v1/nodes/:id/link      # 获取节点分享链接
GET    /api/v1/nodes/:id/qrcode    # 获取二维码 SVG
POST   /api/v1/nodes/regenerate    # 重新生成所有节点（重新运行推荐引擎）
```

### 链式代理

```
POST   /api/v1/chain/generate-code  # 生成配对码（作为落地节点时）
POST   /api/v1/chain/pair           # 配对（作为入口节点时）
GET    /api/v1/chain/peers          # 获取所有落地节点
DELETE /api/v1/chain/peers/:id      # 删除落地节点
GET    /api/v1/chain/rules          # 获取分流规则
POST   /api/v1/chain/rules          # 添加分流规则
PUT    /api/v1/chain/rules/:id      # 修改分流规则
DELETE /api/v1/chain/rules/:id      # 删除分流规则
```

### 订阅

```
GET    /api/v1/subscribe/tokens            # 获取所有订阅 Token
POST   /api/v1/subscribe/tokens            # 创建订阅 Token
DELETE /api/v1/subscribe/tokens/:id        # 删除订阅 Token
GET    /sub/:token                         # 订阅链接（公开，无需认证）
```

### 监控

```
GET    /api/v1/monitor/realtime     # WebSocket 实时流量数据
GET    /api/v1/monitor/traffic      # 流量统计（日/周/月）
GET    /api/v1/monitor/system       # 系统信息（CPU/内存/磁盘）
```

### 系统

```
GET    /api/v1/system/info          # 系统信息
GET    /api/v1/system/update-check  # 检查更新
POST   /api/v1/system/update        # 执行更新
PUT    /api/v1/system/settings      # 修改设置（面板端口、路径等）
```

---

## 前端页面规格

### 全局设计规范

- **主题**: 暗色为主（`zinc-950` 背景），支持亮色切换
- **字体**: `Inter`（英文）+ `Noto Sans SC`（中文）
- **动画**: 所有页面切换和卡片操作使用 Framer Motion，时长 200-300ms，easing `ease-out`
- **圆角**: 卡片 16px，按钮 8px，输入框 8px
- **响应式**: 移动端优先，断点 `sm:640 md:768 lg:1024 xl:1280`
- **颜色系统**:
  - 主色: `emerald-500` (表示运行/健康)
  - 警告: `amber-500`
  - 错误: `red-500`
  - 强调: `violet-500` (推荐标签、重要操作)
  - 背景: `zinc-950` → `zinc-900` → `zinc-800` (层级递进)

### 页面清单

#### 1. Setup 初始化向导
- 全屏居中卡片，Logo + 步骤指示器
- Step 1: 密码输入 + 面板路径（可选展开）
- Step 2: 域名输入 + DNS 验证动画 + 部署进度
- 完成后: 庆祝动画 → 自动跳转 Dashboard

#### 2. Login 登录页
- 全屏居中，极简设计
- 密码输入 + 可选 TOTP 输入
- 错误次数提示

#### 3. Dashboard 主面板
- 顶部: 总流量概览（今日上传/下载，sparkline 小图）
- 主体: 节点卡片网格（响应式 1-3 列）
- 每张卡片: 协议名、标签、状态、延迟、流量、操作按钮
- 底部悬浮: 订阅链接一键复制按钮

#### 4. Chain Proxy 链式代理
- 左侧: 落地节点列表卡片
- 右侧: 分流规则卡片
- 顶部: "生成配对码" / "输入配对码" 按钮

#### 5. Monitor 监控
- 流量趋势图（24h / 7d / 30d 切换）
- 各节点流量占比
- 系统资源使用（CPU / 内存 / 网络）

#### 6. Settings 设置
- 面板设置（端口、路径、HTTPS）
- 安全设置（修改密码、2FA）
- 关于（版本、更新检查）

---

## sing-box 配置生成模板

### VLESS + Reality + Vision

```json
{
  "type": "vless",
  "tag": "vless-reality",
  "listen": "::",
  "listen_port": 443,
  "users": [
    {
      "uuid": "{{UUID}}",
      "flow": "xtls-rprx-vision"
    }
  ],
  "tls": {
    "enabled": true,
    "server_name": "www.microsoft.com",
    "reality": {
      "enabled": true,
      "handshake": {
        "server": "www.microsoft.com",
        "server_port": 443
      },
      "private_key": "{{REALITY_PRIVATE_KEY}}",
      "short_id": ["{{SHORT_ID}}"]
    }
  }
}
```

### Hysteria2

```json
{
  "type": "hysteria2",
  "tag": "hysteria2",
  "listen": "::",
  "listen_port": 8443,
  "up_mbps": 1000,
  "down_mbps": 1000,
  "users": [
    {
      "password": "{{PASSWORD}}"
    }
  ],
  "tls": {
    "enabled": true,
    "alpn": ["h3"],
    "certificate_path": "{{CERT_PATH}}",
    "key_path": "{{KEY_PATH}}"
  }
}
```

### Trojan + TLS

```json
{
  "type": "trojan",
  "tag": "trojan-tls",
  "listen": "::",
  "listen_port": 2083,
  "users": [
    {
      "password": "{{PASSWORD}}"
    }
  ],
  "tls": {
    "enabled": true,
    "certificate_path": "{{CERT_PATH}}",
    "key_path": "{{KEY_PATH}}"
  }
}
```

### VLESS + WS + TLS (CDN 友好)

```json
{
  "type": "vless",
  "tag": "vless-ws-tls",
  "listen": "::",
  "listen_port": 2053,
  "users": [
    {
      "uuid": "{{UUID}}"
    }
  ],
  "transport": {
    "type": "ws",
    "path": "/{{WS_PATH}}"
  },
  "tls": {
    "enabled": true,
    "certificate_path": "{{CERT_PATH}}",
    "key_path": "{{KEY_PATH}}"
  }
}
```

---

## 一键安装脚本规格 (install.sh)

```bash
#!/usr/bin/env bash
# EasyNode 一键安装脚本
# 用法: curl -fsSL https://xxx/install.sh | bash

# 功能:
# 1. 检测系统环境（OS、架构、是否 root）
# 2. 下载对应平台的 easynode 二进制
# 3. 下载 sing-box 二进制
# 4. 创建 systemd 服务文件
# 5. 启动服务
# 6. 输出面板访问地址

# 支持平台:
# - linux/amd64
# - linux/arm64

# 安装目录: /usr/local/easynode/
# 数据目录: /var/lib/easynode/
# 服务名: easynode.service
# 日志: journalctl -u easynode
```

---

## 构建命令 (Makefile)

```makefile
# 构建前端
build-web:
	cd web && pnpm install && pnpm build

# 构建后端（含嵌入前端）
build:
	make build-web
	CGO_ENABLED=0 go build -ldflags="-s -w" -o dist/easynode ./cmd/easynode/

# 交叉编译
build-linux-amd64:
	make build-web
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o dist/easynode-linux-amd64 ./cmd/easynode/

build-linux-arm64:
	make build-web
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o dist/easynode-linux-arm64 ./cmd/easynode/

# 开发模式
dev:
	cd web && pnpm dev &
	air  # Go hot reload
```

---

## 开发里程碑

### Phase 1 — MVP (核心可用)

> 目标: 单机部署、2 个核心协议、卡片 UI、订阅链接

1. Go 项目骨架 + sing-box 进程管理
2. 环境探测引擎
3. ACME 证书自动管理
4. 智能推荐引擎 v1（VLESS Reality + Hysteria2）
5. sing-box 配置自动生成
6. 前端初始化向导
7. 前端 Dashboard + NodeCard 组件
8. 订阅链接生成（V2rayN + Clash 格式）
9. 一键安装脚本
10. 基础安全（JWT + 面板伪装）

### Phase 2 — 完善体验

11. 补全协议（Trojan-TLS, VLESS-WS, TUIC）
12. 链式代理 — 配对码机制
13. 流量统计 + 监控页面
14. 移动端适配
15. 自动更新机制
16. Surge / Shadowrocket 订阅格式

### Phase 3 — 高级功能

17. 多用户管理（轻量级）
18. 智能分流规则
19. 多落地节点负载均衡
20. Telegram Bot 通知
21. WebSocket 实时监控

### Phase 4 — 生态

22. 从 3x-ui 一键迁移工具
23. 插件系统
24. 多语言支持 (i18n)
25. 社区协议模板

---

## 非功能性要求

| 指标 | 要求 |
|------|------|
| 二进制体积 | < 30MB（含前端和 sing-box） |
| 内存占用 | 空闲 < 30MB，运行 < 100MB |
| 启动时间 | < 3 秒 |
| 初始化流程 | < 60 秒完成全部自动配置 |
| 支持系统 | Ubuntu 20.04+, Debian 11+, CentOS 8+, AlmaLinux 8+ |
| 支持架构 | amd64, arm64 |
| 并发连接 | 单节点支持 1000+ 并发 |
| 浏览器 | Chrome 90+, Firefox 90+, Safari 15+, Edge 90+ |
