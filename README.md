# mokoup2p

自托管的 P2P 虚拟组网：设备凭房间号加入同一个虚拟局域网，优先打洞直连，直连不可用时经服务器中转。领域术语见 [GLOSSARY.md](GLOSSARY.md)，架构决策见 [docs/adr](docs/adr)。

## 目录

| 路径 | 内容 |
| --- | --- |
| `api/` | 服务端与客户端共享的 HTTP API 类型和错误码 |
| `server/` | 服务端（Go，SQLite）；`server/servertest` 是接缝 1 测试装置 |
| `client/daemon/` | 客户端后台服务；`client/daemontest` 是接缝 2 测试装置 |
| `client/ipc/` | 后台服务与 CLI / GUI 之间的本地 IPC（Linux 上为 Unix 套接字） |
| `client/engine/` | 网络引擎接口；`easytier` 是生产实现（`easytier-core` 子进程），`enginetest` 是伪造实现 |
| `cmd/mokoup2p-server` | 服务端程序 |
| `cmd/mokoup2pd` | 后台服务程序 |
| `cmd/mokoup2p` | CLI |
| `cmd/mokoup2p-relay` | 中转节点包装进程：同步网络白名单并管理 `easytier-core` |
| `gui/` | Wails + Vue 3 GUI（独立 Go 模块） |
| `deploy/` | Dockerfile、docker-compose、systemd 单元、接缝 3 冒烟测试（`deploy/smoketest`） |

## 部署服务端

```sh
ADMIN_USERNAME=admin ADMIN_PASSWORD=change-me \
MOKOUP2P_RELAY_HOST=p2p.example.com MOKOUP2P_RELAY_TOKEN=$(openssl rand -hex 24) \
docker compose -f deploy/docker-compose.yml up -d
```

docker-compose 启动两个服务：服务端（`server`）和 EasyTier 中转节点（`relay`，锁定 EasyTier v2.6.4）。

| 环境变量 | 说明 |
| --- | --- |
| `ADMIN_USERNAME` / `ADMIN_PASSWORD` | 首次启动时若不存在管理员，按它们创建；之后可以不再提供 |
| `MOKOUP2P_PORT` | 服务端对外端口，默认 8080 |
| `MOKOUP2P_RELAY_HOST` | 中转节点的公网主机名或 IP（下发给设备） |
| `MOKOUP2P_RELAY_PORT` | 中转节点对外端口（TCP 和 UDP），默认 11010 |
| `MOKOUP2P_RELAY_PEERS` | 可选：直接指定下发给设备的中转节点地址（逗号分隔），默认 `tcp://主机:端口,udp://主机:端口` |
| `MOKOUP2P_RELAY_TOKEN` | 服务端与中转节点之间的共享令牌，随便取一个随机串，重启时保持不变 |

中转节点只为本服务端登记过的房间中转：它定期向服务端拉取所有房间的网络名作为 EasyTier 的网络白名单，名单变化时重启自身的 `easytier-core`（见 [ADR-0002](docs/adr/0002-relay-whitelist-sync.md)）。数据保存在 `server-data` 数据卷中。服务端只提供 HTTP，请在前面放置反向代理（如 Caddy）提供 HTTPS。

## 客户端（Linux）

后台服务 `mokoup2pd` 以子进程运行锁定版本（v2.6.4）、未修改的 `easytier-core`，需要 root 权限或 `CAP_NET_ADMIN` 来创建虚拟网卡。从 [EasyTier v2.6.4 发布页](https://github.com/EasyTier/EasyTier/releases/tag/v2.6.4) 下载 `easytier-linux-x86_64-v2.6.4.zip`（sha256 `61b659eaedba658fa66fe47d17e1426cdd77e5d02fa15fed447bb4357c09dfd6`；arm64 为 `easytier-linux-aarch64-v2.6.4.zip`，sha256 `f533ec25a7ea714e09f645615012200278058525795cc3bb690ff011aec1a70f`），把 `easytier-core` 和 `easytier-cli` 放在 `mokoup2pd` 旁边（或用 `MOKOUP2P_EASYTIER_DIR` 指定目录）。

以 systemd 服务运行（实例名是桌面用户的 UID，服务以该用户身份、带 `CAP_NET_ADMIN` 运行）：

```sh
go build -o bin/ ./cmd/mokoup2pd ./cmd/mokoup2p
sudo install -D bin/mokoup2pd /usr/lib/mokoup2p/mokoup2pd
sudo install easytier-core easytier-cli /usr/lib/mokoup2p/
sudo install -m 644 deploy/systemd/mokoup2pd@.service /etc/systemd/system/
sudo systemctl enable --now mokoup2pd@$(id -u)
```

一台机器只能运行一个实例（虚拟网卡名 `mokoup2p0`、端口 11010 和 15888 固定）。

CLI 以普通用户运行：

```sh
bin/mokoup2p server https://p2p.example.com
bin/mokoup2p login -u alice
bin/mokoup2p status
bin/mokoup2p passwd                   # 修改密码
bin/mokoup2p device-name 客厅电脑     # 修改设备名
bin/mokoup2p room create 开黑            # 加 -p 可设置房间密码
bin/mokoup2p room list
bin/mokoup2p join K7QH-3MXP            # 房间有密码时会提示输入
bin/mokoup2p leave
bin/mokoup2p logout
```

用户由管理员在 GUI 的"用户管理"页新增、重置密码、禁用、删除或设为管理员。

## GUI

需要 C 编译器以及 GTK3 / WebKit2GTK 开发包（例如 Fedora：`gcc gtk3-devel webkit2gtk4.1-devel`）。

```sh
cd gui/frontend && npm install && npm run build && cd ..
go build -tags desktop,production,webkit2_41 -o ../bin/mokoup2p-gui .
```

也可以使用 `wails build -tags webkit2_41`。

## 测试

```sh
go test ./...
deploy/smoketest/run.sh   # 接缝 3：真实 EasyTier 端到端冒烟测试（需要 docker 和 /dev/net/tun，不需要 root）
```

升级 EasyTier 版本时必须重新运行冒烟测试。
