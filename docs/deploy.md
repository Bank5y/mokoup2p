# 部署 mokoup2p 服务端

服务端由一个 docker-compose 文件中的两个服务组成：

| 服务 | 作用 | 端口 |
| --- | --- | --- |
| `server` | Go 服务端：用户、房间、成员关系，HTTP JSON API 和 WebSocket。数据存放在 SQLite 中。 | 8080/tcp（只提供 HTTP，放在反向代理后面） |
| `relay` | 锁定版本 v2.6.4 的 `easytier-core` 中转节点，只为服务端登记过的房间中转（ADR-0002）。 | 11010/tcp + 11010/udp（必须对公网开放） |

需要：Docker（带 compose v2）、一个域名或公网 IP。

## 1. 启动

```sh
git clone https://github.com/Bank5y/mokoup2p.git
cd mokoup2p
cat > deploy/.env <<'ENV'
ADMIN_USERNAME=admin
ADMIN_PASSWORD=请换成强密码
MOKOUP2P_RELAY_HOST=p2p.example.com
MOKOUP2P_RELAY_TOKEN=请换成随机字符串
ENV
docker compose -f deploy/docker-compose.yml up -d --build
```

`MOKOUP2P_RELAY_TOKEN` 可以用 `openssl rand -hex 24` 生成。服务端和中转节点用它互相认证，之后不要再改。

首次启动后，用 GUI 或 CLI 以 `ADMIN_USERNAME` / `ADMIN_PASSWORD` 登录，在"管理"页为朋友新增账号。

## 2. 环境变量

| 变量 | 必填 | 说明 |
| --- | --- | --- |
| `ADMIN_USERNAME`、`ADMIN_PASSWORD` | 首次启动 | 数据库中还没有管理员时，用它们创建第一个管理员；已有管理员时被忽略，不会重复创建、也不会改密码。 |
| `MOKOUP2P_RELAY_HOST` | 是 | 中转节点的公网主机名或 IP，会写进下发给设备的入网配置。 |
| `MOKOUP2P_RELAY_PORT` | 否，默认 `11010` | 中转节点对外的 TCP/UDP 端口。 |
| `MOKOUP2P_RELAY_PEERS` | 否 | 直接指定下发给设备的中转地址（逗号分隔，如 `tcp://p2p.example.com:11010,udp://p2p.example.com:11010`），覆盖上面两项拼出的默认值。 |
| `MOKOUP2P_RELAY_TOKEN` | 是 | 中转节点读取房间名单所用的共享令牌。 |
| `MOKOUP2P_PORT` | 否，默认 `8080` | 服务端在宿主机上的 HTTP 端口。放在 Caddy 后面时可以只绑定本机，见下文。 |

不放反向代理时，客户端里的服务端地址直接填 `服务器IP:端口` 即可，例如 `1.2.3.4:8080`。不写协议时客户端按 `http://` 连接，不写端口时按 `8080`。端口改过的话要写上，例如 `1.2.3.4:9000`。注意这种方式是明文 HTTP，登录密码和令牌不加密，只适合内网或测试；公网请按下一节配置 HTTPS。

服务端容器内部的 `MOKOUP2P_LISTEN`（`:8080`）和 `MOKOUP2P_DB`（`/data/mokoup2p.db`）一般不需要改。

每个用户可创建的房间数（默认 5）和每个房间的设备上限（默认 32）不是环境变量，而是系统设置，由管理员在 GUI 的"管理 → 系统设置"中修改。

## 3. HTTPS：Caddy 反向代理示例

服务端只提供 HTTP。客户端登录要传密码和令牌，公网部署请务必在前面放一个提供 HTTPS 的反向代理。Caddy 会自动申请证书并转发 WebSocket。

在 `deploy/` 下新建 `docker-compose.caddy.yml`：

```yaml
services:
  server:
    ports: !override
      - "127.0.0.1:8080:8080"   # 不再直接对外
  caddy:
    image: caddy:2
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
      - "443:443/udp"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy-data:/data
    depends_on:
      - server
volumes:
  caddy-data:
```

`deploy/Caddyfile`：

```
p2p.example.com {
	reverse_proxy server:8080
}
```

启动：

```sh
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.caddy.yml up -d --build
```

客户端里填写的服务端地址为 `https://p2p.example.com`（要写上 `https://`，否则会按 `http://…:8080` 连接）。中转端口 11010 不经过 Caddy：EasyTier 的流量本身已用房间的网络密钥端到端加密。

防火墙需要放行：80/tcp、443/tcp（Caddy），以及 11010/tcp、11010/udp（中转节点）。

## 4. 数据卷与备份

所有数据（用户、设备、房间、虚拟 IP 分配、系统设置）都在 SQLite 数据库 `/data/mokoup2p.db` 中，存放在数据卷 `server-data` 里（compose 项目名为 `deploy` 时卷名是 `deploy_server-data`，可用 `docker volume ls` 查看）。升级或重建容器不会丢数据。成员关系是临时的，服务端重启后不会恢复，设备需要重新加入房间。

备份（数据库使用 WAL 模式，先停服务端再复制，几秒即可）：

```sh
docker compose -f deploy/docker-compose.yml stop server
docker run --rm -v deploy_server-data:/data -v "$PWD":/backup alpine \
  tar czf /backup/mokoup2p-$(date +%F).tar.gz -C /data .
docker compose -f deploy/docker-compose.yml start server
```

恢复：停止服务端，把备份解压回数据卷，再启动：

```sh
docker compose -f deploy/docker-compose.yml stop server
docker run --rm -v deploy_server-data:/data -v "$PWD":/backup alpine \
  sh -c 'rm -rf /data/* && tar xzf /backup/mokoup2p-2026-01-01.tar.gz -C /data'
docker compose -f deploy/docker-compose.yml start server
```

## 5. 升级

```sh
git pull
docker compose -f deploy/docker-compose.yml up -d --build
```

升级 EasyTier 版本（`deploy/Dockerfile.relay`、`packaging/versions.env`）后，请运行 `deploy/smoketest/run.sh` 冒烟测试：EasyTier 的线协议和 RPC 会随版本变化，客户端与中转节点必须使用同一版本。

## 6. 客户端安装包

由 `packaging/build.sh [版本号]` 生成到 `dist/`：

| 平台 | 文件 | 安装 |
| --- | --- | --- |
| Windows x64 | `mokoup2p-<版本>-windows-x64-setup.exe` | 双击安装；注册并启动 Windows 服务 `mokoup2p`，只允许运行安装程序的用户使用；覆盖安装即可升级，GUI 发现新版本时也可一键更新。**需要另行安装 Npcap**（见下文）。卸载：控制面板，或运行安装目录下的 `uninstall.exe`。 |
| Windows x64（绿色版） | `mokoup2p-<版本>-windows-x64-portable.zip` | 解压到可写的目录（例如桌面），双击 `mokoup2p-gui.exe`，确认一次 UAC 即可使用。不注册服务、不写注册表、不开机自启；登录状态和日志在解压目录的 `data\` 中；退出 GUI 即离开房间。不能与安装版同时使用。**同样需要另行安装 Npcap**。 |
| Debian / Ubuntu | `mokoup2p_<版本>_amd64.deb`、`_arm64.deb` | `sudo apt install ./mokoup2p_*.deb`；通过 sudo 安装时自动为当前用户启用 `mokoup2pd@<UID>`。 |
| Fedora / RHEL / openSUSE | `mokoup2p-<版本>.x86_64.rpm`、`.aarch64.rpm` | `sudo dnf install ./mokoup2p-*.rpm` |
| 其他 Linux | `mokoup2p-<版本>-linux-<架构>.tar.gz` | 解压后 `sudo ./install.sh`；卸载 `sudo ./uninstall.sh`（`--purge` 同时删除保存的登录状态）。 |

Linux 上若安装时没有自动启用服务，手动执行 `sudo systemctl enable --now mokoup2pd@$(id -u)`。arm64 包只含后台服务和 CLI（GUI 需要 cgo，目前只构建 amd64）。

## 7. 已知限制

- **同机其他用户可读取网络密钥**：`easytier-core` 的本地 RPC 端口（`127.0.0.1:15888`）只按来源 IP 限制，EasyTier v2.6.4 没有 RPC 鉴权。同一台电脑上的其他本地用户能连上它，读到当前房间的网络密钥。在多人共用的机器上请注意；房主修改房间密码会更换密钥。
- **登录没有限速**：服务端不限制错误密码的尝试次数。公网部署请使用强密码，必要时在反向代理上限流（例如 Caddy 的 rate_limit 插件）。
- **管理员重置密码不会让已登录的设备下线**：如需立即踢下线，请先禁用再启用该用户。

## 8. 第三方软件与许可（EasyTier，LGPL-3.0）

客户端以独立子进程运行**未修改的** EasyTier v2.6.4 官方发布二进制（`easytier-core`、`easytier-cli`），只通过命令行、环境变量和本地 RPC 与之交互，不链接、不修改 EasyTier。按 LGPL-3.0：

- 安装包附带 EasyTier 的许可全文（`EasyTier-LICENSE.txt`）和来源说明（`THIRD_PARTY_NOTICES.md`）；
- 源代码见 https://github.com/EasyTier/EasyTier/tree/v2.6.4；
- 用户可以用自己构建的同版本二进制替换它们（放在 `mokoup2pd` 旁边，或用 `MOKOUP2P_EASYTIER_DIR` 指定目录）。

中转节点镜像使用官方镜像 `easytier/easytier:v2.6.4`（按 digest 锁定）。

Windows 安装包还带有 Wintun（`wintun.dll`，按其预编译二进制许可原样分发，附 `wintun-LICENSE.txt`）和 WinDivert 驱动（`WinDivert64.sys`，LGPL-3.0 / GPL-2.0）。**Npcap 的 `Packet.dll` 不随安装包分发**：Npcap 的许可不允许再分发，请从 https://npcap.com 下载安装（免费版限 5 台机器），原因和后续替换计划见 `docs/adr/0004-npcap-dependency.md`。
