# Windows 广播修复复用 EasyTier 内置的 `--enable-udp-broadcast-relay`，不自研、不复用 WinIPBroadcast

Windows 上从 `0.0.0.0` 发往 `255.255.255.255` 的 UDP 广播只走首选路由所在的那块网卡（通常是物理网卡），到不了 EasyTier 的虚拟网卡，房间里的其他成员就发现不了局域网游戏。发往虚拟网段广播地址和组播的包本来就能被 EasyTier 泛洪，缺口只在受限广播。

EasyTier v2.6.4 自带修复：`--enable-udp-broadcast-relay true`（仅 Windows，需要管理员权限）。它用 WinDivert 以**嗅探**模式抓本机物理网卡地址发出的受限广播、物理网段的定向广播和组播，把源地址改成本设备的虚拟 IP（定向广播改成虚拟网段广播），再送进房间；原始包照常发往物理网卡。这个功能随虚拟网卡一起启动、随 `easytier-core` 进程一起结束，所以离开房间后修复自动失效，物理网络的广播也不受影响。后台服务在 Windows 上通过引擎选项 `UDPBroadcastRelay` 打开它；后台服务本来就以 LocalSystem 运行，满足权限要求。我们不写任何抓包代码，许可上只多了 EasyTier 压缩包里的 `WinDivert64.sys`（LGPLv3 / GPLv2）。

## Considered Options

- **复用 WinIPBroadcast**：GPL-3.0，不能并入本项目；只能借鉴思路（在 127.0.0.1 上开原始套接字接收本机广播的回环副本再转发）。它还收不到源端口和目的端口相同的包。
- **自研 Go 版"WinIPBroadcast"**：约 300 行，有同样的端口限制；只在内置功能对某款游戏不起作用时再考虑。
- **给 255.255.255.255/32 加一条指向虚拟网卡的低跃点路由**：WinIPBroadcast 的作者说明这种方式"时灵时不灵"；路由存在期间物理局域网收不到受限广播，进程崩溃还会留下残留路由。
- **Npcap 抓包**：Npcap 的许可不允许再分发。

## Consequences

- v2.6.4 有一个已知问题：每个受限或定向广播到达对端两次（WinDivert 把回环副本也当作出站包）。上游已在 v2.7.0 修复（EasyTier#2619），v2.7.0 正式发布后升级。游戏发现协议通常能容忍重复包。
- 修复只认启动时的物理网卡地址：在房间中切换 Wi-Fi 网络后，可能要重新进入房间。
- 需要在真实 Windows 机器上手动验证（`docs/windows.md`）；Linux 不在本修复范围内（Linux 上应用发往虚拟网段广播或绑定虚拟网卡即可）。
