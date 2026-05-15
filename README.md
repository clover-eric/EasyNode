# EasyNode

EasyNode 是一个面向普通用户的智能代理节点面板。目标是让用户不再纠结 VLESS、Reality、TLS、WebSocket、Hysteria2、TUIC 等底层选择，只需要输入域名或选择 IP 直连，系统自动生成推荐节点。

默认界面支持中文和 English，可在页面右上角切换。

## 主要功能

- Go 单二进制程序，内置 Web 面板
- 中英文界面切换
- 首次访问初始化向导
- 智能推荐节点方案：VLESS Reality、Hysteria2、Trojan TLS、VLESS WS TLS、TUIC
- 用普通用户能理解的方式说明每种节点：推荐度、适用场景、适配软件、优缺点
- 订阅链接生成
- sing-box 配置生成
- 链式代理配对码流程
- 设置页面：修改管理员密码、域名、IP 直连模式、面板路径
- 数据持久化到数据目录

## 一行安装

推荐一行安装：

```bash
curl -fsSL https://raw.githubusercontent.com/clover-eric/EasyNode/main/scripts/install.sh | bash -s -- --yes --repo clover-eric/EasyNode
```

脚本会优先下载 GitHub Release 里的 `easynode-linux-amd64` 或 `easynode-linux-arm64`。如果还没有发布 Release，会自动 fallback 到源码构建。

交互式安装：

```bash
curl -fsSL https://raw.githubusercontent.com/clover-eric/EasyNode/main/scripts/install.sh | bash -s -- --repo clover-eric/EasyNode
```

安装脚本会自动：

- 检测系统和 CPU 架构
- 可选升级系统软件包
- 可选安装常用依赖：`curl`、`ca-certificates`、`tar`、`gzip`、`git`、`make`
- 可选启用 BBR 加速
- 创建数据目录并设置权限
- 安装 systemd 服务
- 设置开机自启
- 在启用 `ufw` 或 `firewalld` 时开放面板端口
- 启动后自检服务是否可用
- 只输出一个浏览器访问地址

常用参数：

```bash
# 指定端口
curl -fsSL https://raw.githubusercontent.com/clover-eric/EasyNode/main/scripts/install.sh | bash -s -- --yes --repo clover-eric/EasyNode --port 8088

# 跳过系统升级和 BBR
curl -fsSL https://raw.githubusercontent.com/clover-eric/EasyNode/main/scripts/install.sh | bash -s -- --yes --repo clover-eric/EasyNode --skip-upgrade --skip-bbr
```

## 服务器源码测试

你也可以手动源码构建测试：

```bash
apt update && apt install -y git golang make
git clone https://github.com/clover-eric/EasyNode.git
cd EasyNode
make build
bash scripts/install.sh --yes --skip-upgrade --skip-bbr
```

然后打开安装脚本输出的面板地址。

## 本地运行

```bash
go run ./cmd/easynode -addr :8088 -data data
```

访问：

```text
http://127.0.0.1:8088
```

## 安全卸载

默认卸载会停止服务、删除二进制和 systemd 文件，并把数据目录备份到 `/var/lib/easynode-uninstall-时间`：

```bash
curl -fsSL https://raw.githubusercontent.com/clover-eric/EasyNode/main/scripts/uninstall.sh | bash
```

如果确认不要保留数据：

```bash
curl -fsSL https://raw.githubusercontent.com/clover-eric/EasyNode/main/scripts/uninstall.sh | bash -s -- --purge
```

## 构建

```bash
make build
make build-linux-amd64
make build-linux-arm64
```

构建输出：

```text
dist/easynode
dist/easynode-linux-amd64
dist/easynode-linux-arm64
```

## 当前状态

当前版本是可运行 MVP，已经适合测试面板流程、节点推荐逻辑、订阅生成和基础部署流程。

生产环境还需要继续补齐：

- ACME 自动证书
- 真实 sing-box 进程托管和热重载
- 真实 DNS/IP/端口/UDP 探测
- SQLite 存储
- 跨服务器链式代理密钥握手
- Release 自动构建和校验文件
