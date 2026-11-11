# 数据面采用 EasyTier（三层 + 广播泛洪），而非真正的二层

需求最初是类似 ZeroTier 的二层虚拟以太网，但我们选用 EasyTier 作为数据面内核：它只做三层（TUN），把 IPv4 广播和组播泛洪给全部节点，能覆盖大多数基于 UDP 广播的局域网游戏发现，但不支持 IPX 等非 IP 协议。之所以接受这个取舍，是因为 EasyTier 全部以 LGPL-3.0 开源（含配置服务器），社区活跃，中转节点可以自托管，Go 服务端只需下发网络名、网络密钥和虚拟 IP 即可掌控它；客户端以独立进程运行未修改的 `easytier-core` 二进制，LGPL 义务最小。

## Considered Options

- **ZeroTierOne**：真正的二层，但控制器（`nonfree/`）采用仅限非商业的源码可见许可，与"开源、他人自托管"的定位冲突；绕开它需要用 C++ 重写 netconf 控制器协议。
- **自研（pion/ice + pion/turn + TAP）**：二层和许可都没问题，但打洞、中转、调优全部自研，违背"使用已有开源内核"的初衷。

## Consequences

- Windows 上发往 255.255.255.255 的广播默认走物理网卡，需要额外处理（类似 WinIPBroadcast）。
- EasyTier 的线协议和 RPC 随版本变化，需要锁定使用的 EasyTier 版本。
