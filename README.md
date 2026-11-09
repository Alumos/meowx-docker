# MeowX Docker 版

在 NAS 上用 Docker 跑 MeowX：局域网设备、NAS 上的其它容器都可以用 NAS 的代理端口（缺省 7890，HTTP / SOCKS5）；
还能加入主控的喵窝组网，在外面访问家里的 NAS 和局域网。全部在浏览器里管理（`http://<NAS 的 IP>:7899`）。

以绿联 **UGOS Pro** 为主，其它 x86 Linux NAS 同样能用。从 [MeowX 飞牛版](https://github.com/mmwx-group/meowx-fnos) 移植而来。

- 订阅：粘贴订阅链接、上传配置，或登录主控账户后从账户导入；自动更新、流量 / 到期显示
- 节点：分组、测速、切换；规则 / 全局 / 直连；连接与日志
- 代理端口：HTTP / SOCKS5 混合端口，局域网免密、外网连进来要密码
- 喵窝：官方 tailscaled 入网，可以开放 NAS 所在的局域网网段；与系统自带的 Tailscale 不冲突
- po0 / GGY / AkileCDN 加白

## 部署

```bash
mkdir -p meowx && cd meowx
curl -fsSLO https://raw.githubusercontent.com/Alumos/meowx-docker/main/docker/docker-compose.yaml
docker compose up -d
```

- 绿联 UGOS Pro 的图形界面部署、离线镜像包、迁移与排查：见 [docs/install-ugos.md](docs/install-ugos.md)
- 镜像：`ghcr.io/alumos/meowx-docker`（`latest` = 最新正式版，`edge` = main 最新提交）。只有 amd64。
- 拉不到 GHCR 时：到 [Releases](https://github.com/Alumos/meowx-docker/releases) 下载 `meowx-docker-<版本>-amd64.tar.gz`，`docker load -i` 导入

要求：x86_64，Docker 带 compose；容器用 host 网络与 `NET_ADMIN`、`/dev/net/tun`（喵窝要）。

**网页密码**：缺省不要密码，7899 端口会开放到外网时请在「设置」里设一个。

**更新**：`docker compose pull && docker compose up -d`。设置、订阅、喵窝身份都在 `./data`，不会丢。

## 构建

全部在 GitHub Actions 里（[.github/workflows/build.yml](.github/workflows/build.yml)，`ubuntu-latest`）：推 main 出 `edge` 镜像；
改 `VERSION` 后打 `v<版本>` tag，出正式镜像与 Release。

本地要构建的话：`scripts/build-rootfs.sh <版本> && docker build -t meowx-docker docker`（要 Go 1.27、git、curl）。
内核从 [meowx-core](https://github.com/mmwx-group/meowx-core) 按 `scripts/build-core.sh` 里固定的提交编译；喵窝用官方 tailscale 静态程序（`scripts/fetch-tailscale.sh`，校验 sha256）。

设计见 [docs/design.md](docs/design.md)，控制服务的 API 见 [docs/api.md](docs/api.md)。

## 致谢与许可

基于 [meowx-fnos](https://github.com/mmwx-group/meowx-fnos)；内核是 MeowX 的 mihomo 分支 [meowx-core](https://github.com/mmwx-group/meowx-core)；
后台做法来自 [MeowX OpenWrt 版](https://github.com/mmwx-group/meowx-openwrt)（基于 [OpenWrt-nikki](https://github.com/nikkinikki-org/OpenWrt-nikki)）。
本项目以 [GPL-3.0](LICENSE) 开源。
