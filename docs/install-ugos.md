# 在绿联 UGOS Pro 上部署

## 准备

1. 应用中心装好 **Docker**。
2. 在文件管理里建一个目录放 MeowX，例如 `共享文件夹/docker/meowx`（下面叫「项目目录」）。设置、订阅、喵窝身份都会存在它下面的 `data/`。

## 用 Docker 项目部署（推荐）

1. Docker → **项目** → **创建**。
2. 名称填 `meowx`，存放路径选上面的项目目录。
3. 来源选「创建 docker-compose.yml」，把 [docker-compose.yaml](../docker/docker-compose.yaml) 的内容整个粘进去。
4. 点「立即部署」。第一次要拉镜像 `ghcr.io/alumos/meowx-docker:latest`（约 30 MB）。
5. 浏览器打开 `http://<NAS 的 IP>:7899`。

拉不动镜像（GHCR 在国内有时很慢）时，用下面的离线方式。

## 离线镜像包

1. 到 [Releases](https://github.com/Alumos/meowx-docker/releases) 下载 `meowx-docker-<版本>-amd64.tar.gz`。
2. Docker → **镜像** → **添加** → **从本地导入**，选这个文件（不用解压）。
3. 再按上一节创建项目；镜像已经在本机，不会再去拉。

## 用 SSH 部署

```bash
mkdir -p /volume1/docker/meowx && cd /volume1/docker/meowx
curl -fsSLO https://raw.githubusercontent.com/Alumos/meowx-docker/main/docker/docker-compose.yaml
sudo docker compose up -d
```

（`/volume1/docker` 换成你自己的路径。）

## 用起来

1. **订阅**页：粘贴订阅链接，或用主控的登录链接登录、从账户导入。
2. **概览**页点「启动」。
3. 局域网里的设备、NAS 上的其它容器，把代理设成 `http://<NAS 的 IP>:7890`（SOCKS5 也是这个端口）。
   局域网免密；从外网连进来要用「混入设置」里的账号密码（首次启动随机生成）。
4. **喵窝**：在「订阅」页登录主控账户后，到「喵窝」页打开开关、勾选要开放的局域网网段。页面下方的「系统检查」应当全是正常。
5. 7899 端口会开放到外网时，在「设置」里设网页访问密码。

## 更新

网页「设置」里检查更新。有新版时，在 Docker → 项目 → meowx 里重新拉取镜像并重启，或者 SSH 执行：

```bash
cd /volume1/docker/meowx && sudo docker compose pull && sudo docker compose up -d
```

离线：下载新版离线镜像包导入，再重启项目。`data/` 不受影响。

## 和 UGOS 自带的 Tailscale 一起用

MeowX 的喵窝缺省用网卡 `meowx-ts`、UDP 41642，与 Tailscale 应用（`tailscale0`、41641）错开，两个可以同时开。
要改就改 compose 里的 `MEOWX_TS_DEVICE` / `MEOWX_TS_PORT`。

## 从飞牛版迁移

把飞牛上的 `/vol1/@appdata/meowx/data` 整个拷到项目目录的 `data/`（先停掉两边的 MeowX），再启动。订阅、设置、主控账户、喵窝身份都会保留。
同一个喵窝身份不要在两台机器上同时在线。

## 排查

- 网页打不开：Docker → 容器 → meowx → 日志；或 `sudo docker logs meowx`。
- 7890 / 7899 被占用：`sudo ss -lntp | grep -E '7890|7899'`，改「混入设置」里的端口或 compose 里的 `MEOWX_UI_PORT`。
- 喵窝开放的网段访问不了：看「喵窝 → 系统检查」。IP 转发没打开时在宿主执行 `sudo sysctl -w net.ipv4.ip_forward=1`。
