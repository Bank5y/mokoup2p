# mokoup2p

自托管的 P2P 虚拟组网：设备凭房间号加入同一个虚拟局域网，优先打洞直连，直连不可用时经服务器中转。领域术语见 [GLOSSARY.md](GLOSSARY.md)，架构决策见 [docs/adr](docs/adr)。

## 目录

| 路径 | 内容 |
| --- | --- |
| `api/` | 服务端与客户端共享的 HTTP API 类型和错误码 |
| `server/` | 服务端（Go，SQLite）；`server/servertest` 是接缝 1 测试装置 |
| `client/daemon/` | 客户端后台服务；`client/daemontest` 是接缝 2 测试装置 |
| `client/ipc/` | 后台服务与 CLI / GUI 之间的本地 IPC（Linux 上为 Unix 套接字） |
| `client/engine/` | 网络引擎接口；`enginetest` 是伪造实现 |
| `cmd/mokoup2p-server` | 服务端程序 |
| `cmd/mokoup2pd` | 后台服务程序 |
| `cmd/mokoup2p` | CLI |
| `gui/` | Wails + Vue 3 GUI（独立 Go 模块） |
| `deploy/` | Dockerfile 与 docker-compose |

## 部署服务端

```sh
ADMIN_USERNAME=admin ADMIN_PASSWORD=change-me docker compose -f deploy/docker-compose.yml up -d
```

首次启动时若不存在管理员，会按 `ADMIN_USERNAME` / `ADMIN_PASSWORD` 创建；之后可以不再提供。数据保存在 `server-data` 数据卷中。服务端只提供 HTTP，请在前面放置反向代理（如 Caddy）提供 HTTPS。

## 客户端（Linux）

```sh
go build -o bin/ ./cmd/...
bin/mokoup2pd &                       # 后台服务
bin/mokoup2p server https://p2p.example.com
bin/mokoup2p login -u alice
bin/mokoup2p status
bin/mokoup2p room create 开黑            # 加 -p 可设置房间密码
bin/mokoup2p room list
```

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
```
