# MeowX Docker 版设计

> 2026-11-09 从 [meowx-fnos](https://github.com/mmwx-group/meowx-fnos) 0.1.2（`991ba4b`）复制出来的独立仓库。
> 目标：绿联 UGOS Pro 为主，其它 x86 Linux NAS 用 `docker compose up -d` 部署。只做 amd64。

## 1. 和飞牛版的区别

| | 飞牛版 | Docker 版 |
|---|---|---|
| 分发 | fpk，镜像在 NAS 上就地构建（`FROM scratch`） | 预构建镜像 `ghcr.io/alumos/meowx-docker`（alpine）+ Release 里的离线镜像包 |
| 起停 | 飞牛应用中心按 `docker-project` 起停 | 用户自己的 compose（UGOS：Docker → 项目） |
| 透明代理（NAS 本机 / 容器 / 旁路由） | 有 | **去掉**：只用代理端口（混合端口 7890，HTTP + SOCKS5） |
| 喵窝 | tailscale0 / 41641 | 网卡、端口可配（缺省 `meowx-ts` / 41642，与系统自带 Tailscale 错开）；防火墙在没有 nf_tables 时改用 iptables-legacy；Docker 用 iptables-legacy 时往 legacy 的 DOCKER-USER 放行 |
| 更新 | 经飞牛应用中心原地升级 | 只检查 GitHub Releases，网页给出 `docker compose pull && docker compose up -d` 与离线镜像包 |
| 构建 | 自建 runner，x86 + arm | GitHub Actions `ubuntu-latest`，只 amd64 |

其余（设置 / 订阅 / 主控账户 / 运行配置合成 / 内核看管 / po0 / GGY / AkileCDN / 网页）与飞牛版一致，数据目录格式相同：
飞牛版的 `/vol1/@appdata/meowx/data` 整个拷到 Docker 版的 `./data` 即可接着用。

## 2. 组成

```
ghcr.io/alumos/meowx-docker（alpine:3.22 + ca-certificates tzdata iptables iptables-legacy）
  /usr/bin/meowx-agent   控制服务（agent/，Go，PID 1）：网页 + API、生成运行配置、看管内核与 tailscaled、喵窝防火墙
  /usr/bin/meowx-core    内核（meowx-core 独立程序，tag with_gvisor,no_easytier,no_tailscale,no_zerotier）
  /usr/bin/tailscaled、tailscale   官方静态程序（BSD-3）
  /usr/share/meowx/geo/  随镜像带的 geo 数据（首次启动复制到 /data/run）
```

容器跑在 host 网络，`NET_ADMIN` + `NET_RAW` + `/dev/net/tun`（喵窝要）。数据都在 `/data`（挂 `./data`）。

## 3. 端口

| 端口 | 用途 |
|---|---|
| 7890/tcp | 混合代理端口：局域网与本机免密，外网来的要密码（混入设置里的账号） |
| 7899/tcp | 网页控制台（`MEOWX_UI_PORT`） |
| 1053 | 内核 DNS（只在本机用，可关） |
| 127.0.0.1:9090 | 内核外部控制（只给控制服务用，网页经 `/api/core/*` 转） |
| 41642/udp | 喵窝直连（`MEOWX_TS_PORT`） |

## 4. 环境变量

| 变量 | 缺省 | 说明 |
|---|---|---|
| `MEOWX_UI_PORT` | 7899 | 网页端口 |
| `MEOWX_DATA` | /data | 数据目录 |
| `MEOWX_TS_DEVICE` | meowx-ts | 喵窝网卡名（≤15 字符） |
| `MEOWX_TS_PORT` | 41642 | 喵窝 UDP 端口 |
| `MEOWX_MESH_OS` / `MEOWX_MESH_CLIENT` | fnos / meowx-fnos | 入网时报给主控的系统 / 客户端；主控支持别的值以后再改 |
| `MEOWX_MESH_ROUTER_PREFIX` | fnos | 新生成的喵窝路由器标识前缀（已有的不变） |
| `MEOWX_UPDATE_REPO` | Alumos/meowx-docker | 检查更新读哪个仓库的 Releases |

## 5. 喵窝的防火墙

- 先用 nftables（表 `inet meowx_mesh`：masquerade、映射段 1:1 DNAT、可选禁止访问 NAS 本身）；建表失败（内核没有 nf_tables）时用
  iptables-legacy 写同样的规则（自己的链 `MEOWX_MESH_POST` / `MEOWX_MESH_PRE` / `MEOWX_MESH_IN`，NETMAP 做映射）。
- Docker 的 FORWARD 缺省丢弃：在 DOCKER-USER 里插两条放行（进来的、回包）。nft 里有 DOCKER-USER（Docker 用 iptables-nft）就插在那，
  legacy 表里有（Docker 用 iptables-legacy）就用 iptables-legacy 插。关掉 / 容器停止时只删这两条；启动时先清一遍上次残留的。
- 网页「喵窝 → 系统检查」显示 TUN、ip_forward、防火墙后端、DOCKER-USER 在哪。

## 6. 构建与发布

`.github/workflows/build.yml`：gofmt / vet / test → `scripts/build-rootfs.sh`（编 agent 与内核、取 tailscale 与 geo）→ `docker build docker/`
→ 推 GHCR（main：`edge`；tag：`<版本>` + `latest`）→ tag 时 `docker save` 出离线包，连同 docker-compose.yaml、SHA256SUMS 发 Release。
版本号在仓库根目录 `VERSION`，打的 tag 要是 `v<VERSION>`。
