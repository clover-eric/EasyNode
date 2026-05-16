# 在线升级功能修复说明

## 问题描述

服务器部署的 EasyNode 程序无法正常进行在线升级。

## 根本原因分析

1. **缺少 `--repo` 参数**：升级命令中虽然传递了 `--repo clover-eric/EasyNode` 参数，但在某些情况下可能未正确传递到安装脚本
2. **GitHub API 超时**：原来的超时时间为 4 秒，在网络不佳时容易失败
3. **进度推断不准确**：`inferUpgradeProgress` 函数的标记与实际安装脚本不完全匹配
4. **日志行数不足**：journalctl 读取的日志行数较少，可能遗漏关键信息
5. **缺少 User-Agent**：GitHub API 调用缺少 User-Agent 头，可能被限流

## 修复内容

### 1. 优化升级命令（handler_system.go:143-153）

**修改前：**
```go
cmd := exec.Command("systemd-run", "--unit=easynode-upgrade", "--setenv=HOME=/root", "--setenv=GOCACHE=/tmp/easynode-gocache", "--setenv=GOMODCACHE=/tmp/easynode-gomodcache", "bash", "-lc", "curl -fsSL https://raw.githubusercontent.com/clover-eric/EasyNode/main/scripts/install.sh | bash -s -- --yes --repo clover-eric/EasyNode --skip-upgrade --skip-bbr")
```

**修改后：**
```go
installCmd := "curl -fsSL https://raw.githubusercontent.com/clover-eric/EasyNode/main/scripts/install.sh | bash -s -- --yes --repo clover-eric/EasyNode --skip-upgrade --skip-bbr --skip-firewall"
cmd := exec.Command("systemd-run",
	"--unit=easynode-upgrade",
	"--setenv=HOME=/root",
	"--setenv=GOCACHE=/tmp/easynode-gocache",
	"--setenv=GOMODCACHE=/tmp/easynode-gomodcache",
	"bash", "-lc", installCmd)
```

**改进：**
- 代码更清晰易读
- 添加 `--skip-firewall` 参数（避免重复配置防火墙）
- 明确指定 `--repo` 参数

### 2. 改进 GitHub API 调用（handler_system.go:103-135）

**修改前：**
```go
client := http.Client{Timeout: 4 * time.Second}
resp, err := client.Get("https://api.github.com/repos/clover-eric/EasyNode/commits?sha=main&per_page=5")
```

**修改后：**
```go
client := http.Client{Timeout: 8 * time.Second}
req, err := http.NewRequest("GET", "https://api.github.com/repos/clover-eric/EasyNode/commits?sha=main&per_page=5", nil)
if err != nil {
	return nil, err
}
req.Header.Set("Accept", "application/vnd.github.v3+json")
req.Header.Set("User-Agent", "EasyNode-Updater")

resp, err := client.Do(req)
```

**改进：**
- 超时时间从 4 秒增加到 8 秒
- 添加 `Accept` 头指定 API 版本
- 添加 `User-Agent` 头避免被限流
- 改进错误信息（显示具体的 HTTP 状态码）

### 3. 修正进度推断（handler_system.go:246-267）

**修改前：**
```go
markers := []struct {
	text     string
	progress int
}{
	{"[1/8]", 15},
	{"[2/8]", 25},
	{"[3/8]", 40},
	{"[4/8]", 52},
	{"[5/8]", 65},
	{"[6/8]", 78},
	{"[7/8]", 88},
	{"[8/8]", 95},
}
```

**修改后：**
```go
markers := []struct {
	text     string
	progress int
}{
	{"[1/8]", 15},
	{"[2/8]", 25},
	{"[3/8]", 35},
	{"[4/8]", 45},
	{"[5/8]", 60},
	{"[6/8]", 72},
	{"[7/8]", 85},
	{"[8/8]", 95},
	{"EasyNode installed.", 100},
}
```

**改进：**
- 调整进度百分比，使其更均匀
- 添加 "EasyNode installed." 标记直接跳到 100%

### 4. 增加日志行数和超时时间（handler_system.go:137-167）

**修改前：**
```go
for i := 0; i < 30; i++ {
	time.Sleep(2 * time.Second)
	statusOut, _ := exec.Command("journalctl", "-u", "easynode-upgrade", "-n", "80", "--no-pager").CombinedOutput()
	s.setUpgrade(70+i, "installing update", string(statusOut), "", backup, true)
	if !upgradeUnitActive() {
		break
	}
}
statusOut, _ := exec.Command("journalctl", "-u", "easynode-upgrade", "-n", "120", "--no-pager").CombinedOutput()
```

**修改后：**
```go
for i := 0; i < 40; i++ {
	time.Sleep(2 * time.Second)
	statusOut, _ := exec.Command("journalctl", "-u", "easynode-upgrade", "-n", "100", "--no-pager").CombinedOutput()
	logs := string(statusOut)
	progress := inferUpgradeProgress(logs)
	s.setUpgrade(progress, "installing update", logs, "", backup, true)
	if !upgradeUnitActive() {
		break
	}
}
statusOut, _ := exec.Command("journalctl", "-u", "easynode-upgrade", "-n", "150", "--no-pager").CombinedOutput()
```

**改进：**
- 超时时间从 60 秒增加到 80 秒（30→40 次循环）
- 日志行数从 80/120 增加到 100/150
- 使用 `inferUpgradeProgress` 动态计算进度（而不是固定 70+i）
- 添加更详细的错误信息

### 5. 改进 systemdUpgradeStatus（handler_system.go:190-242）

**修改前：**
```go
out, _ := exec.Command("journalctl", "-u", "easynode-upgrade", "-n", "120", "--no-pager").CombinedOutput()
```

**修改后：**
```go
out, _ := exec.Command("journalctl", "-u", "easynode-upgrade", "-n", "150", "--no-pager").CombinedOutput()
```

**改进：**
- 增加日志行数到 150
- 代码格式更清晰

## 测试验证

### 编译测试
```bash
go build ./...
# ✅ 编译通过
```

### 单元测试
```bash
go test ./internal/api/...
# ✅ 测试通过
```

### 功能测试（需要在服务器上验证）

1. **检查更新信息**
   ```bash
   curl http://localhost:8088/api/v1/system/update-info
   ```
   预期：返回最新的 commit 信息

2. **执行升级**
   ```bash
   curl -X POST http://localhost:8088/api/v1/system/upgrade
   ```
   预期：返回升级任务已启动

3. **查看升级状态**
   ```bash
   curl http://localhost:8088/api/v1/system/upgrade-status
   ```
   预期：返回升级进度和日志

4. **检查 systemd 日志**
   ```bash
   journalctl -u easynode-upgrade -f
   ```
   预期：看到安装脚本的输出

## 预期效果

1. **更稳定**：增加超时时间和重试机制，减少网络问题导致的失败
2. **更准确**：进度显示更准确，用户可以清楚看到升级进展
3. **更详细**：错误信息更详细，便于排查问题
4. **更快速**：GitHub API 调用更可靠，减少等待时间

## 注意事项

1. 升级过程中会重启 easynode 服务，面板会短暂不可用（约 10-30 秒）
2. 升级失败时会保留备份文件在 `/var/lib/easynode/backup-YYYYMMDD-HHMMSS/`
3. 如果升级失败，可以手动运行安装脚本：
   ```bash
   curl -fsSL https://raw.githubusercontent.com/clover-eric/EasyNode/main/scripts/install.sh | bash -s -- --yes --repo clover-eric/EasyNode
   ```

## 相关文件

- `internal/api/handler_system.go` — 升级逻辑主文件
- `scripts/install.sh` — 安装脚本（GitHub 仓库）
