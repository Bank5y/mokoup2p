# Windows 客户端

## 组成

| 部分 | 说明 |
| --- | --- |
| `mokoup2pd.exe` | Windows 服务 `mokoup2p`（LocalSystem，自动启动，失败后自动重启）。`install [-user 用户名]` 注册并启动，`uninstall` 停止并删除，`run` 在控制台运行（调试）。 |
| IPC | 命名管道 `\\.\pipe\mokoup2p`。ACL 为 `D:P(A;;GA;;;SY)(A;;GRGW;;;<用户 SID>)`：只有 SYSTEM 和安装时指定的用户能连接。服务以 `-ipc-user <SID>` 参数注册。 |
| 状态 | `%ProgramData%\mokoup2p\state.json`（刷新令牌、设备标识），日志 `mokoup2pd.log`。 |
| `easytier-core.exe` | 锁定的 v2.6.4，未修改，作为子进程运行，放进一个 "关闭即终止" 的作业对象：服务进程无论怎样退出，子进程都会随之结束；Wintun 网卡随进程消失。启动时若 PID 文件指向的进程仍是 `easytier-core.exe`，先将其结束（残留清理）。 |
| GUI | Wails 在 Windows 上使用 WebView2，不需要 cgo；托盘可用，点 × 隐藏到托盘。 |

## 第三方文件与许可

EasyTier 的 Windows 压缩包包含：

- `wintun.dll`：Wintun 预编译二进制许可，可原样再分发（需附带其 LICENSE.txt）。
- `WinDivert64.sys`：LGPLv3 / GPLv2，广播修复（#13）使用。
- `Packet.dll`：**Npcap 的库，其许可不允许未经许可再分发**。`easytier-core.exe` v2.6.4 硬依赖它（上游 issue EasyTier#2273）。

在这个问题解决之前，安装包不能内置 `Packet.dll`。可选方案：用户自行安装 Npcap；安装时从 EasyTier 官方发布页下载压缩包到用户机器上（我们不再分发它）；或获得 Npcap 的再分发许可。

## 手动验证（#12 验收）

以下步骤无法在 Linux 开发环境中自动执行，需要在真实的 Windows 10/11 x64 机器上完成：

1. 以管理员身份运行 `mokoup2pd.exe install`。`sc query mokoup2p` 显示 RUNNING；`%ProgramData%\mokoup2p\mokoup2pd.log` 有 `listening on \\.\pipe\mokoup2p`。
2. 以安装用户运行 `mokoup2p.exe status`：能连上后台服务。换一个本机用户运行：连接被拒绝。
3. `mokoup2p.exe server …`、`login`、`join <房间号>`，同一房间里有一台 Linux 设备：
   - 两边互相 `ping` 对方的虚拟 IP；
   - `mokoup2p.exe members` 显示对方为"直连"或"中转"以及延迟。
4. 在任务管理器里结束 `mokoup2pd.exe`：`easytier-core.exe` 随之消失，网络适配器列表中没有残留的 `mokoup2p0`；服务自动重启后不会自动回到房间。
5. 运行 GUI：登录、加入房间、成员列表、点 × 隐藏到托盘仍在房间中、托盘"退出"离开房间。
6. `mokoup2pd.exe uninstall`：服务删除，虚拟网卡消失。

## 广播修复（#13 验收）

后台服务在 Windows 上给 `easytier-core.exe` 加上 `--enable-udp-broadcast-relay true`（见 ADR-0003），需要 `WinDivert64.sys` 与 `easytier-core.exe` 放在一起。

1. Windows 设备 A 和 Linux 或 Windows 设备 B 进入同一房间。
2. 在 B 上监听 UDP 端口并打印来源地址，例如 Linux：`socat -u UDP4-RECVFROM:9999,reuseaddr,fork SYSTEM:'echo $SOCAT_PEERADDR'`。
3. 在 A 上从 `0.0.0.0` 向 `255.255.255.255:9999` 发送广播（例如 PowerShell：`$u=New-Object Net.Sockets.UdpClient; $u.EnableBroadcast=$true; $b=[Text.Encoding]::ASCII.GetBytes('hi'); $u.Send($b,$b.Length,'255.255.255.255',9999)`）。
   B 应收到来源为 **A 的虚拟 IP** 的包（v2.6.4 下每个包会到两次，属已知问题）。
4. 同一时间物理局域网上的其他机器也应照常收到这个广播（修复不拦截原始包）。
5. A 离开房间后再发：B 不再收到。
6. 用至少一款依赖 UDP 受限广播发现房间的局域网游戏（或上面的测试工具）在 A 与 B 之间互相发现。
