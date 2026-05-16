# EasyNode 协议设计分析与优化方案

## 一、现有协议评估

### 1. Shadowsocks (chacha20-ietf-poly1305)
**优点：**
- 轻量级，CPU 占用低
- 兼容性好，客户端广泛
- 适合低延迟场景

**问题：**
- 特征明显，易被识别
- 无混淆，抗封锁能力弱
- 建议：保留但不作为主推协议

### 2. VLESS-Reality
**优点：**
- 无特征，伪装成正常 TLS 流量
- 性能优秀，延迟低
- 抗封锁能力强

**问题：**
- 当前实现固定 SNI 为 `www.microsoft.com`，缺少灵活性
- 建议：支持自定义 SNI 和 dest

**优化方案：**
```go
// 添加到 model.Node
RealityDest string `json:"reality_dest,omitempty"` // 默认 www.microsoft.com:443
RealitySNI  string `json:"reality_sni,omitempty"`  // 默认 www.microsoft.com
```

### 3. Trojan-TLS
**优点：**
- 伪装成 HTTPS 流量
- 兼容性好

**问题：**
- 需要真实证书，维护成本高
- 性能不如 Reality
- 建议：保留但优先推荐 Reality

### 4. Hysteria2
**优点：**
- 基于 QUIC，拥塞控制优秀
- 适合高丢包环境
- 速度快

**问题：**
- UDP 流量特征明显
- 部分网络环境 UDP 被限速
- 建议：作为备选协议

### 5. VLESS-WS-TLS
**优点：**
- WebSocket 伪装，CDN 友好
- 可复用 443 端口

**问题：**
- 性能开销大（WebSocket 封装）
- 延迟高于直连 TLS
- 建议：仅在需要 CDN 时使用

### 6. TUIC
**优点：**
- 基于 QUIC，性能好
- 拥塞控制优秀

**问题：**
- 客户端支持有限
- UDP 特征明显
- 建议：作为实验性协议

---

## 二、Clash 配置重构方案

### 当前问题
1. **规则硬编码** — 只有 10 几条域名规则，无法覆盖常见场景
2. **DNS 配置不合理** — fake-ip 可能导致国内网站解析错误
3. **代理组单一** — 缺少地区、媒体、广告拦截等分组
4. **无规则集支持** — 无法动态更新规则

### 重构目标
1. 使用规则集（rule-set）实现精准分流
2. 优化 DNS 配置（分流 DNS + 国内直连）
3. 设计合理的代理组层级
4. 支持自定义规则
5. 性能优化

### 新架构设计

#### 1. DNS 配置（分流 DNS）
```yaml
dns:
  enable: true
  listen: 0.0.0.0:1053
  ipv6: true
  enhanced-mode: fake-ip
  fake-ip-range: 198.18.0.1/16
  fake-ip-filter:
    - '*.lan'
    - '*.local'
    - '+.msftconnecttest.com'
    - '+.msftncsi.com'
  
  # 国内 DNS（直连）
  nameserver:
    - https://223.5.5.5/dns-query
    - https://doh.pub/dns-query
  
  # 国外 DNS（代理）
  proxy-server-nameserver:
    - https://223.5.5.5/dns-query
  
  nameserver-policy:
    # 国内域名使用国内 DNS
    'geosite:cn': [https://doh.pub/dns-query, https://223.5.5.5/dns-query]
    # 国外域名使用代理 DNS
    'geosite:geolocation-!cn': [https://1.1.1.1/dns-query, https://8.8.8.8/dns-query]
```

#### 2. 代理组设计
```
┌─────────────┐
│   GLOBAL    │  用户选择入口
└──────┬──────┘
       │
   ┌───┴────┬────────┬──────────┐
   │        │        │          │
┌──▼──┐ ┌──▼──┐ ┌───▼───┐ ┌────▼────┐
│Proxy│ │China│ │AdBlock│ │Streaming│
└──┬──┘ └─────┘ └───────┘ └────┬────┘
   │                            │
┌──▼──────────┐          ┌─────▼──────┐
│ Auto/Fallback│          │Netflix/YouTube│
└──────────────┘          └────────────┘
```

**代理组说明：**
- **GLOBAL** — 全局策略选择器
- **Proxy** — 代理节点组（自动选择/故障转移）
- **China** — 国内直连
- **AdBlock** — 广告拦截（REJECT）
- **Streaming** — 流媒体专用（Netflix、YouTube 等）
- **Auto** — 自动选择最快节点（url-test）
- **Fallback** — 故障转移（fallback）

#### 3. 规则集架构
使用 Clash Meta 的 rule-providers：

```yaml
rule-providers:
  reject:
    type: http
    behavior: domain
    url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/reject.txt"
    path: ./ruleset/reject.yaml
    interval: 86400
  
  direct:
    type: http
    behavior: domain
    url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/direct.txt"
    path: ./ruleset/direct.yaml
    interval: 86400
  
  proxy:
    type: http
    behavior: domain
    url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/proxy.txt"
    path: ./ruleset/proxy.yaml
    interval: 86400
  
  gfw:
    type: http
    behavior: domain
    url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/gfw.txt"
    path: ./ruleset/gfw.yaml
    interval: 86400
  
  cncidr:
    type: http
    behavior: ipcidr
    url: "https://cdn.jsdelivr.net/gh/Loyalsoldier/clash-rules@release/cncidr.txt"
    path: ./ruleset/cncidr.yaml
    interval: 86400
```

#### 4. 规则优先级
```yaml
rules:
  # 1. 局域网直连
  - IP-CIDR,10.0.0.0/8,DIRECT,no-resolve
  - IP-CIDR,172.16.0.0/12,DIRECT,no-resolve
  - IP-CIDR,192.168.0.0/16,DIRECT,no-resolve
  - GEOIP,private,DIRECT,no-resolve
  
  # 2. 广告拦截
  - RULE-SET,reject,AdBlock
  
  # 3. 流媒体
  - DOMAIN-SUFFIX,netflix.com,Streaming
  - DOMAIN-SUFFIX,youtube.com,Streaming
  
  # 4. 国内直连
  - RULE-SET,direct,China
  - GEOIP,cn,China,no-resolve
  - RULE-SET,cncidr,China,no-resolve
  
  # 5. 国外代理
  - RULE-SET,proxy,Proxy
  - RULE-SET,gfw,Proxy
  
  # 6. 兜底规则
  - MATCH,GLOBAL
```

#### 5. 性能优化参数
```yaml
# 全局配置
mixed-port: 7890
allow-lan: true
mode: rule
log-level: warning
ipv6: true

# 性能优化
unified-delay: true          # 统一延迟测试
tcp-concurrent: true         # TCP 并发
find-process-mode: strict    # 进程匹配
global-client-fingerprint: chrome

# 缓存
profile:
  store-selected: true       # 保存选择
  store-fake-ip: true        # 保存 fake-ip 映射

# 实验性特性
experimental:
  sniff-tls-sni: true       # 嗅探 TLS SNI
```

---

## 三、实现计划

### Phase 1: 重构 Clash 生成器
- [ ] 创建 `internal/core/subscribe/clash_v2.go`
- [ ] 实现规则集下载和缓存
- [ ] 实现分流 DNS 配置
- [ ] 实现代理组层级

### Phase 2: 优化协议配置
- [ ] Reality 支持自定义 SNI/dest
- [ ] 添加协议优先级推荐
- [ ] 优化 sing-box 配置生成

### Phase 3: 测试和文档
- [ ] 编写单元测试
- [ ] 性能基准测试
- [ ] 用户文档

---

## 四、配置示例对比

### 旧配置（当前）
- 规则数：~15 条
- 代理组：4 个（Proxy/Auto/Fallback/Global）
- DNS：fake-ip 单一模式
- 规则集：无

### 新配置（重构后）
- 规则数：~10000+ 条（通过规则集）
- 代理组：7+ 个（分层设计）
- DNS：分流 DNS + nameserver-policy
- 规则集：5 个（reject/direct/proxy/gfw/cncidr）
- 更新：自动更新规则集（24h）

---

## 五、性能预期

### 延迟优化
- 自动选择：每 5 分钟测速，选择最快节点
- 故障转移：节点失败自动切换
- 并发连接：tcp-concurrent 减少握手延迟

### 分流准确性
- 国内网站：100% 直连（基于 geosite:cn + cncidr）
- 国外网站：精准代理（基于 gfw + proxy 规则集）
- 广告拦截：~90% 覆盖率（基于 reject 规则集）

### 资源占用
- 内存：+10MB（规则集缓存）
- CPU：无明显增加（规则匹配优化）
- 磁盘：+5MB（规则集文件）
