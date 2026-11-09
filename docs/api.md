# 控制服务 API（网页界面用）

全部在 `http://<NAS>:7899/api/…`（Docker 版），JSON。出错时 HTTP 4xx/5xx + `{"error": "中文原因"}`；
有些操作「做成了但套用失败」时是 200 + `{"…": …, "error": "原因"}`，界面要把 `error` 显示出来。

设置了网页密码（`settings.web.password`）时，除 `GET /api/status`、`POST /api/login` 外都要先登录（Cookie `meowx_session`），没登录回 401。

## 状态与服务

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/status` | `{version, need_login, enabled, profile, core: {running, started_at, error, restarts}, error, mixed_port}`；`need_login` 为真时只有前两项 |
| POST | `/api/login` | `{password}` → 设 Cookie |
| POST | `/api/service` | `{action: "start"\|"stop"\|"restart"}`；start / stop 会改开关并存盘 |
| GET | `/api/settings` | 整份设置（结构见 `agent/internal/store/settings.go`，JSON 键是 snake_case） |
| PUT | `/api/settings` | 整份替换；默认存完立刻套用（`?apply=0` 只存）。回 `{settings, error?}` |
| GET | `/api/runconfig` | 当前在跑的运行配置（YAML 文本，只读看） |
| GET | `/api/logs?after=<序号>&source=core\|agent` | 日志增量：`[{seq, time(ms), source, text}]`，界面每 2 秒拿一次 `after=上次最大 seq` |
| DELETE | `/api/logs` | 清空 |
| GET | `/api/update[?refresh=1]` | 检查新版本（GitHub Releases `Alumos/meowx-docker` 的 latest，缓存一小时）：`{current, latest, has_update, released_at, notes_url, download_url（离线镜像包）, size, can_install（恒为 false）, command（更新命令）, image, error, checked_at}` |

## 宿主

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/host` | `{interfaces: {Physical: [{name, subnets, up}], Bridges: [...]}, checks: Checks}` |

Checks：`{tun, ip_forward, nftables, firewall（nftables / iptables-legacy）, docker_user（nftables / iptables-legacy / none）, device, port}`，喵窝页的「系统检查」用。
Docker 版没有透明代理：`PUT /api/settings` 里的 `proxy.enabled` 一律存成 false，`/api/proxy*` 已去掉。

## 订阅

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/profiles` | `{profiles: [Profile], active: "<id>"}` |
| POST | `/api/profiles` | `{url, name?, use?}` 添加并下载；`use` 或还没有在用的就切过去。回 `{profile, error?}` |
| POST | `/api/profiles/upload?name=&use=1` | 正文是配置文件原文（YAML） |
| PATCH | `/api/profiles/{id}` | `{name?, url?, info_url?, user_agent?}` |
| DELETE | `/api/profiles/{id}` | 删的是在用的那份时停内核（同退出登录） |
| POST | `/api/profiles/{id}/update` | 重新下载；在用且在跑时重载。回 `{profile, error?}`（失败时旧副本保留，`profile.error` 也有原因） |
| POST | `/api/profiles/{id}/use` | 切过去（没开的顺手开）；起不来就退回原来的选择 |
| GET | `/api/profiles/{id}/content` | 订阅原文 |

Profile：`{id, name, url?, info_url?, user_agent?, from_panel?, updated_at(秒), update_interval(小时), used, total, expire(秒), error?}`。
`url` 为空 = 本地上传的、不能刷新。`from_panel` = 从主控账户导入的（退出登录时会删）。

## 主控账户（经 `meowx-core panel …`，返回体就是它的 JSON 原样）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/panel` | 登录状态（`panel status`） |
| POST | `/api/panel/login` | `{link}`：主控网页「登录客户端」给的 `miaomiaowu://login?…` 链接。回 `panel login` 的 JSON，另加 `imported`（登录时自动导入的订阅：当前没在用订阅、账户下恰好一份时才有）或 `import_error`；登录后在后台拉一次 po0 列表、补报一次白名单 |
| GET | `/api/panel/profile` | 向主控重取昵称 / 头像（`panel profile`），只在用户点「刷新」时调 |
| POST | `/api/panel/logout` | 退出；删掉从主控导入的订阅（在用的删掉后停内核） |
| GET | `/api/panel/subscriptions` | 账户下的订阅（`panel subscriptions`） |
| POST | `/api/panel/import` | `{url, name}` 导入账户下的一份并切过去 |
| POST | `/api/panel/po0` | `{action: status\|enable\|disable\|fetch\|add\|remove, address?, token?, index?}` |

## GGY 加白

`POST /api/panel/ggy` `{action: status|enable|disable|add|remove|report, token?, index?}`：与主控账户无关、没登录也能用；`token` 可以是整条分享链接。

## AkileCDN 白名单

`POST /api/panel/akile` `{action, key?, port?, rule_id?, name?, label?, index?}`，回 `meowx-core panel akile …` 的 JSON（Key 只回脱敏写法）。
与主控账户无关、没登录也能用；规格同各端（五端共用规格 §8）。

| action | 参数 | 说明 |
|---|---|---|
| `status` / `enable` / `disable` | | 打开开关时当场查出口 IP、把全部节点写一次 |
| `setkey` / `clearkey` | `key` | 合法（`ak_` 开头）才存；开着时换 Key 当场写一次 |
| `nodes` | | 订阅里的可选节点：内核在跑读运行配置，没在跑读选中的订阅 |
| `rules` | `port` | 用存着的 Key 查监听这个端口的规则（直接问 Akile，不改设置） |
| `add` | `rule_id, port, name, label` | 开关开着时当场写这一个 |
| `remove` | `index` | 删除不触发上报 |
| `report` | | 开关开着才报 |

设置存 `/data/panel/akile.json`（`MEOWX_AKILE_STATE`），每个节点上一次的结果存 `/data/panel/akile-results.json`（`MEOWX_AKILE_RESULT`，
控制服务启动时删掉，等同 OpenWrt 版的内存盘）。定时上报（同 po0 / GGY，内核在跑时每 10 分钟）带 `--timer`：出口 IP 与上一次写入的相同就跳过；
控制服务启动后的第一轮一定写。

## 喵窝

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/mesh` | `{mesh: View, lan: ["10.10.1.0/24", …], checks: Checks}`；`lan` 是可开放的局域网网段候选 |
| PUT | `/api/mesh` | 喵窝设置 `{enabled, force_map, host_access, networks, hostname}`：存好按设置入网 / 退出（最多一两分钟）。回 `{ok}` 或 `{error}` |
| POST | `/api/mesh/leave` | 退出喵窝：主控里删这台设备、撤掉申报的网段、关开关。回 `{ok, warning?}` |

View：`{settings, hostname, state: {router_id, control_url, magic_dns, sites: [{original, advertised, mapped}], advertised, hosts, error, applied_at},
panel_logged_in, daemon, backend("Running"/"NeedsLogin"…), self: Peer, peers: [Peer], control_url, advertised}`；
Peer：`{name, dns_name, ip, os, online, relay, direct, routes}`。没登录主控时入网会失败并提示先登录；`host_access` = 喵窝里的设备能不能访问 NAS 本身（缺省开）。

## 内核 API（节点、连接、规则、日志）

`/api/core/*` 原样转给内核的外部控制（mihomo REST API，WebSocket 也转），例如：
`GET /api/core/proxies`、`PUT /api/core/proxies/<组>` `{name}`、`GET /api/core/proxies/<节点>/delay?url=…&timeout=5000`、
`GET /api/core/connections`（或 WebSocket）、`DELETE /api/core/connections/<id>`、`GET /api/core/rules`、`PATCH /api/core/configs` `{mode}`。
内核没在跑时回 502 `{"error":"内核没在运行"}`。
