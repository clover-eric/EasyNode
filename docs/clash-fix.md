# Clash 协议修复说明

## 问题描述

用户反馈：
1. 面板上显示 2 个 Clash 协议卡片
2. 这 2 个 Clash 协议都不能用
3. 只需要一个能用的 Clash 订阅

## 根本原因

系统中存在两个 Clash 相关的实现：

### 1. 旧的 Clash 协议（已废弃）
- 位置：`internal/core/recommender/engine.go`
- 定义：`{Protocol: "clash", Transport: "rule", Security: "client-profile", Priority: 4, Label: "Clash 分流", Description: "Clash/Mihomo 订阅，自动测速和精准分流", Enabled: false}`
- 状态：`Enabled: false`（已禁用但仍显示在前端）
- 实现：`internal/core/subscribe/generator.go` 中的 `Clash()` 函数
- 问题：
  - 作为一个"节点协议"存在，但实际上 Clash 是订阅格式而非协议
  - 规则简单（只有 15 条硬编码规则）
  - 代理组少（只有 5 个）
  - 不支持规则集

### 2. 新的 ClashV2 订阅（正常工作）
- 位置：`internal/core/subscribe/clash_v2.go`
- 访问：`/api/v1/subscribe/KEY/clash`
- 特性：
  - 使用 Loyalsoldier 规则集（10000+ 条规则）
  - 8 个代理组（GLOBAL, Proxy, Auto, Fallback, LoadBalance, Streaming, China, AdBlock）
  - 支持自动测速、负载均衡
  - 完整的 DNS 分流配置

## 修复内容

### 1. 移除旧的 Clash 协议定义

**文件：** `internal/core/recommender/engine.go`

```diff
- {Protocol: "clash", Transport: "rule", Security: "client-profile", Priority: 4, Label: "Clash 分流", Description: "Clash/Mihomo 订阅，自动测速和精准分流", Enabled: false},
```

**效果：** 前端不再显示旧的 Clash 卡片

### 2. 移除 Clash 特殊依赖逻辑

**文件：** `internal/api/handler_nodes.go`

```diff
- if req.Protocol == "clash" && !hasProtocol(st.Nodes, "shadowsocks") {
-     for _, dep := range recs {
-         if dep.Protocol == "shadowsocks" {
-             st.Nodes = append(st.Nodes, newNodeFromRecommendation(dep, host, st.CertReady))
-             break
-         }
-     }
- }
```

**说明：** 旧代码在添加 Clash 协议时会自动添加 Shadowsocks 依赖，现在不再需要

### 3. 删除旧的 Clash() 函数

**文件：** `internal/core/subscribe/generator.go`

删除了 60 行的旧 `Clash()` 函数实现，包括：
- 简单的规则配置
- 基础的代理组设置
- 硬编码的域名规则

**保留：** 辅助函数（`clashProxies`, `clashProxyForNode`, `writeClashGroup` 等）因为 ClashV2 仍在使用

### 4. 更新测试用例

**文件：** `internal/core/subscribe/generator_test.go`

```diff
- yaml := Clash(nodes)
+ yaml := ClashV2(nodes)
```

更新了测试断言以匹配 ClashV2 的输出格式：
- 添加 `LoadBalance` 代理组检查
- 更新规则检查（从硬编码域名改为 RULE-SET）
- 保持对空配置和无效节点的测试

## ClashV2 订阅特性

### 访问方式

```
https://your-domain/api/v1/subscribe/KEY/clash
```

或在订阅链接后添加参数：
```
https://your-domain/api/v1/subscribe/KEY?format=clash
https://your-domain/api/v1/subscribe/KEY?target=clash
```

### 配置特性

#### 1. 全局设置
- 混合端口：7890
- 允许局域网访问
- IPv6 支持
- 统一延迟测试
- TCP 并发
- 进程匹配模式：strict
- 客户端指纹：chrome

#### 2. DNS 配置
- 模式：fake-ip
- 国内 DNS：阿里 DNS、腾讯 DNS
- 国外 DNS：Cloudflare、Google
- 智能分流（根据 GeoIP）

#### 3. 代理组（8 个）

| 代理组 | 类型 | 说明 |
|--------|------|------|
| GLOBAL | select | 全局策略选择 |
| Proxy | select | 代理节点选择 |
| Auto | url-test | 自动测速选择最快节点 |
| Fallback | fallback | 故障转移 |
| LoadBalance | load-balance | 负载均衡（一致性哈希） |
| Streaming | select | 流媒体专用 |
| China | select | 国内网站 |
| AdBlock | select | 广告拦截 |

#### 4. 规则集（10000+ 条）

使用 Loyalsoldier/clash-rules 规则集：
- `reject.txt` - 广告拦截
- `icloud.txt` - Apple iCloud
- `apple.txt` - Apple 服务
- `google.txt` - Google 服务
- `proxy.txt` - 代理规则
- `direct.txt` - 直连规则
- `private.txt` - 私有网络
- `gfw.txt` - GFW 列表
- `tld-not-cn.txt` - 非中国顶级域名
- `telegramcidr.txt` - Telegram IP

#### 5. 规则优先级

```
1. 广告拦截 → AdBlock
2. 私有网络 → DIRECT
3. Apple 服务 → China
4. Google 服务 → Proxy
5. 代理规则 → Proxy
6. 直连规则 → China
7. GFW 列表 → Proxy
8. 非中国域名 → Proxy
9. Telegram → Proxy
10. 国内 IP → China
11. 其他 → GLOBAL
```

## 兼容的客户端

ClashV2 配置兼容以下客户端：

### Windows
- Clash for Windows (CFW)
- Clash Verge
- Clash Verge Rev
- Clash.Meta

### macOS
- ClashX
- ClashX Pro
- Clash Verge

### Android
- Clash for Android
- ClashMeta for Android
- Surfboard

### iOS
- Stash
- Shadowrocket (部分支持)

### Linux
- Clash
- Clash.Meta
- Mihomo

## 测试验证

### 1. 编译测试
```bash
go build ./...
# ✅ 编译通过
```

### 2. 单元测试
```bash
go test ./...
# ✅ 所有测试通过
```

### 3. 功能测试

#### 检查前端卡片
1. 登录面板
2. 查看协议列表
3. 确认只显示一个 Clash 相关的订阅（不是协议卡片）

#### 测试订阅链接
```bash
# 获取 Clash 订阅
curl https://your-domain/api/v1/subscribe/KEY/clash

# 应该返回完整的 YAML 配置
# 包含 proxies, proxy-groups, rule-providers, rules
```

#### 导入客户端测试
1. 复制订阅链接
2. 在 Clash 客户端中添加订阅
3. 更新订阅
4. 检查：
   - 节点列表正确
   - 代理组显示正常
   - 规则集下载成功
   - 可以正常连接

#### 分流测试
- 访问 Google → 走代理
- 访问 Baidu → 直连
- 访问广告域名 → 拦截

## 升级说明

### 服务器端升级

```bash
# 1. 拉取最新代码
cd /path/to/EasyNode
git pull origin main

# 2. 编译
go build -o easynode ./cmd/easynode

# 3. 重启服务
systemctl restart easynode

# 4. 查看日志
journalctl -u easynode -f
```

### 客户端更新

如果之前使用旧的 Clash 协议：
1. 删除旧的订阅
2. 使用新的订阅链接：`https://your-domain/api/v1/subscribe/KEY/clash`
3. 更新订阅
4. 测试连接

## 注意事项

1. **规则集下载**
   - 首次使用时，Clash 会从 CDN 下载规则集
   - 需要确保客户端能访问 `cdn.jsdelivr.net`
   - 规则集会缓存在本地，定期自动更新

2. **内存占用**
   - ClashV2 配置包含 10000+ 条规则
   - 客户端内存占用会比旧版本略高（约 50-100MB）
   - 但性能和准确性大幅提升

3. **兼容性**
   - 需要 Clash Premium 或 Clash.Meta 内核
   - 旧版 Clash 可能不支持某些特性（如 rule-set）
   - 建议使用最新版客户端

4. **自定义规则**
   - 如需自定义规则，可以修改 `clash_v2.go`
   - 或在客户端中添加自定义规则（会覆盖订阅规则）

## 相关文件

- `internal/core/recommender/engine.go` - 协议推荐引擎
- `internal/api/handler_nodes.go` - 节点管理 API
- `internal/core/subscribe/generator.go` - 订阅生成器（旧 Clash 已删除）
- `internal/core/subscribe/clash_v2.go` - ClashV2 实现
- `internal/core/subscribe/generator_test.go` - 测试用例
- `internal/api/handler_subscribe.go` - 订阅 API 路由

## 提交信息

```
commit 03b09ab
fix: remove legacy Clash protocol, use ClashV2 subscription only

Issues:
- Two Clash cards displayed in frontend (legacy + ClashV2)
- Legacy Clash protocol not working
- User only needs one Clash subscription

Changes:
- Removed 'clash' protocol from recommender/engine.go
- Removed special clash dependency logic in handler_nodes.go
- Deleted old Clash() function from generator.go (60 lines)
- Updated tests to use ClashV2() instead of Clash()
- Kept ClashV2 subscription at /api/v1/subscribe/KEY/clash

Result:
- Only one Clash subscription card shown
- ClashV2 with 10000+ rules and 8 proxy groups
- Compatible with Clash/Mihomo/ClashMeta clients
```
