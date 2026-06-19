# VPS Monitor API 文档

本服务连接 Xray 的多个 gRPC service，并将其能力包装为 HTTP JSON API：

| Service | gRPC 包 | 能力 |
|---|---|---|
| `StatsService` | `app/stats/command` | 统计查询：6 个方法 1:1 映射 + 业务聚合端点 |
| `HandlerService` | `app/proxyman/command` | inbound 用户管理：增删用户（仅 VLESS）、列举、计数 |
| `RoutingService` | `app/router/command` | 只读路由规则列举 |

---

## 1. 基础信息

| 项 | 说明 |
|---|---|
| Base URL | `http://<host>:8090`（监听地址由 `WEB_LISTEN` 决定） |
| 响应类型 | `application/json; charset=utf-8`，缩进美化输出 |
| 字符编码 | UTF-8，不转义 HTML（`<`/`>`/`&` 原样输出） |
| 字节字段 | 数值字段（如 `value`、`uplink`）为原始字节整数；同名 `*_human` 字段为可读字符串，如 `10.00 MB`（单位 B/KB/MB/GB/TB） |
| 时间字段 | `*_unix` 为 Unix 秒；同名去后缀字段为本地时区可读串，格式 `2006-01-02 15:04:05` |
| 请求超时 | 每个请求查询 Xray 的超时为 5 秒 |

### 配置（环境变量）

| 变量 | 说明 | 默认值 |
|---|---|---|
| `XRAY_API_ADDR` | Xray gRPC API 服务端地址 | `67.209.178.119:80` |
| `WEB_LISTEN` | 本服务 HTTP 监听地址 | `:8090` |
| `ADMIN_TOKEN` | 写操作鉴权 token，**为空时所有写操作返回 403** | （空） |
| `HOURLY_DATA_FILE` | 每小时流量持久化 JSON 文件路径 | `./data/hourly.json` |
| `SAMPLE_INTERVAL` | 每小时流量后台采样间隔（秒） | `60` |
| `RETENTION_DAYS` | 每小时流量历史保留天数 | `30` |

启动：`go run ./cmd/vps`，或编译后 `./vps`。

---

## 2. 认证

仅**写操作**（`POST` / `DELETE`，即增删用户）需要认证。在请求头携带：

```
Authorization: Bearer <ADMIN_TOKEN>
```

- `ADMIN_TOKEN` 未配置（空）→ 所有写操作返回 `403`，提示「写操作未启用」。
- token 缺失或不匹配 → 返回 `403`。
- 只读端点（所有 `GET`）无需认证。

---

## 3. 错误处理

错误响应统一为：

```json
{ "error": "错误描述" }
```

| 状态码 | 含义 | 触发场景 |
|---|---|---|
| `200` | 成功 | 正常返回 |
| `400` | 请求有误 | 缺少必填查询参数、JSON body 无法解析、写操作缺少必填字段 |
| `403` | 鉴权失败 | 写操作 token 缺失/错误，或未配置 `ADMIN_TOKEN` |
| `500` | 服务端错误 | gRPC 调用 Xray 失败（如连接断开、tag 不存在） |

---

## 4. 公共查询参数

| 参数 | 类型 | 说明 |
|---|---|---|
| `name` | string | 计数器/用户名称；单计数器端点必填 |
| `pattern` | string | 计数器名称匹配前缀，空表示匹配全部 |
| `reset` | bool | 是否在读取后重置计数器；仅 `true` 或 `1` 视为真，其余（含缺省）为假 |
| `tag` | string | inbound 的 tag；用户管理端点必填 |

Xray 计数器名称约定为 `category>>>name>>>traffic>>>direction`，例如
`user>>>foo>>>traffic>>>uplink`。聚合端点据此解析，不符合格式的计数器会被忽略。

---

## 5. StatsService — 统计查询

### 5.1 GET /api/sysstats

系统运行状态。无参数。

```bash
curl http://localhost:8090/api/sysstats
```

```json
{
  "uptime_seconds": 123456,
  "num_goroutine": 31,
  "num_gc": 12,
  "alloc_bytes": 10485760,
  "alloc_human": "10.00 MB",
  "total_alloc_bytes": 52428800,
  "total_alloc_human": "50.00 MB",
  "sys_bytes": 73400320,
  "sys_human": "70.00 MB"
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `uptime_seconds` | int | Xray 运行时长（秒） |
| `num_goroutine` | int | 当前 goroutine 数 |
| `num_gc` | int | GC 次数 |
| `alloc_bytes` / `alloc_human` | int / string | 当前堆分配 |
| `total_alloc_bytes` / `total_alloc_human` | int / string | 累计堆分配 |
| `sys_bytes` / `sys_human` | int / string | 向系统申请的内存 |

### 5.2 GET /api/stats

查询单个计数器取值。

| 参数 | 必填 | 说明 |
|---|---|---|
| `name` | 是 | 计数器名称 |
| `reset` | 否 | 读取后是否重置 |

```bash
curl 'http://localhost:8090/api/stats?name=user>>>foo>>>traffic>>>uplink'
```

```json
{
  "name": "user>>>foo>>>traffic>>>uplink",
  "value": 10485760,
  "value_human": "10.00 MB"
}
```

### 5.3 GET /api/stats/online

查询单个计数器的在线数（如用户在线连接数）。

| 参数 | 必填 | 说明 |
|---|---|---|
| `name` | 是 | 计数器名称 |
| `reset` | 否 | 读取后是否重置 |

```bash
curl 'http://localhost:8090/api/stats/online?name=user>>>foo>>>online'
```

```json
{
  "name": "user>>>foo>>>online",
  "value": 3
}
```

### 5.4 GET /api/stats/online/iplist

查询单个用户的在线来源 IP 及最后活跃时间，按 IP 排序。

| 参数 | 必填 | 说明 |
|---|---|---|
| `name` | 是 | 用户名 |

```bash
curl 'http://localhost:8090/api/stats/online/iplist?name=foo'
```

```json
[
  {
    "ip": "1.2.3.4",
    "last_seen_unix": 1718700000,
    "last_seen": "2024-06-18 12:00:00"
  }
]
```

### 5.5 GET /api/query

返回原始（未聚合）计数器列表。

| 参数 | 必填 | 说明 |
|---|---|---|
| `pattern` | 否 | 名称前缀，空表示全部 |
| `reset` | 否 | 读取后是否重置 |

```bash
curl 'http://localhost:8090/api/query?pattern=user>>>'
```

```json
[
  {
    "name": "user>>>foo>>>traffic>>>uplink",
    "value": 10485760,
    "value_human": "10.00 MB"
  },
  {
    "name": "user>>>foo>>>traffic>>>downlink",
    "value": 20971520,
    "value_human": "20.00 MB"
  }
]
```

### 5.6 GET /api/online/users

返回排序后的在线用户名列表。无参数。

```bash
curl http://localhost:8090/api/online/users
```

```json
["alice", "bob", "foo"]
```

---

## 6. StatsService — 业务聚合端点

### 6.1 GET /api/traffic

把名称形如 `category>>>name>>>traffic>>>direction` 的计数器按 `category → name`
聚合上下行，分类固定顺序为 user / inbound / outbound。

```bash
curl http://localhost:8090/api/traffic
```

```json
[
  {
    "category": "user",
    "label": "用户 (user)",
    "entries": [
      {
        "name": "foo",
        "uplink": 10485760,
        "uplink_human": "10.00 MB",
        "downlink": 20971520,
        "downlink_human": "20.00 MB",
        "total": 31457280,
        "total_human": "30.00 MB"
      }
    ]
  }
]
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `category` | string | 分类标识：`user` / `inbound` / `outbound` |
| `label` | string | 分类中文标签 |
| `entries[].name` | string | 对象名（用户名 / inbound tag / outbound tag） |
| `entries[].uplink` / `downlink` / `total` | int | 上行 / 下行 / 合计字节，及对应 `*_human` |

### 6.1.1 GET /api/traffic/hourly

返回**每小时流量**序列。Xray 计数器只有累计值，本服务在后台按 `SAMPLE_INTERVAL`
（默认 60s）采样并差分，按整点小时桶累加，落盘到 `HOURLY_DATA_FILE`（默认
`./data/hourly.json`），保留最近 `RETENTION_DAYS`（默认 30）天。

> 注意：服务重启后首次采样仅重建基线，重启瞬间到首次采样之间的流量不计入。
> 当前未结束的小时桶也会返回，代表"本小时到此刻的累计"。小时边界按服务器本地时区对齐。

查询参数：

| 参数 | 说明 |
|---|---|
| `hours` | 回溯最近 N 小时（正整数，默认 24）。与 `from`/`to` 互斥，后者优先。 |
| `from` / `to` | 时间范围，接受 RFC3339（如 `2026-06-19T00:00:00Z`）或 Unix 秒。只给 `from` 时 `to` 默认当前。 |
| `category` | 可选，仅返回该类：`user` / `inbound` / `outbound`。 |
| `name` | 可选，配合 `category` 仅返回该对象（用户名 / tag）。 |

```bash
curl 'http://localhost:8090/api/traffic/hourly?hours=24'
curl 'http://localhost:8090/api/traffic/hourly?category=user&name=foo&hours=6'
```

```json
[
  {
    "hour_unix": 1750334400,
    "hour": "2026-06-19T12:00:00+08:00",
    "categories": [
      {
        "category": "user",
        "label": "用户 (user)",
        "entries": [
          {
            "name": "foo",
            "uplink": 1048576,
            "uplink_human": "1.00 MB",
            "downlink": 2097152,
            "downlink_human": "2.00 MB",
            "total": 3145728,
            "total_human": "3.00 MB"
          }
        ]
      }
    ]
  }
]
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `hour_unix` | int | 该小时整点的 Unix 秒 |
| `hour` | string | 该小时整点的 RFC3339（服务器本地时区） |
| `categories` | array | 与 `/api/traffic` 同构（含 `*_human`），但数值为**该小时净流量**而非累计 |

### 6.2 GET /api/online

返回在线用户及其来源 IP。单个用户的 IP 查询失败不影响整体，其 `ips` 置空。

```bash
curl http://localhost:8090/api/online
```

```json
[
  {
    "user": "foo",
    "ips": [
      {
        "ip": "1.2.3.4",
        "last_seen_unix": 1718700000,
        "last_seen": "2024-06-18 12:00:00"
      }
    ]
  }
]
```

### 6.3 GET /api/all

`sysstats` + `traffic` + `online` 三合一。

```bash
curl http://localhost:8090/api/all
```

```json
{
  "sys_stats": {
    "uptime_seconds": 123456,
    "num_goroutine": 31,
    "num_gc": 12,
    "alloc_bytes": 10485760,
    "alloc_human": "10.00 MB",
    "total_alloc_bytes": 52428800,
    "total_alloc_human": "50.00 MB",
    "sys_bytes": 73400320,
    "sys_human": "70.00 MB"
  },
  "traffic": [
    {
      "category": "user",
      "label": "用户 (user)",
      "entries": [
        {
          "name": "foo",
          "uplink": 10485760,
          "uplink_human": "10.00 MB",
          "downlink": 20971520,
          "downlink_human": "20.00 MB",
          "total": 31457280,
          "total_human": "30.00 MB"
        }
      ]
    }
  ],
  "online": [
    {
      "user": "foo",
      "ips": [
        {
          "ip": "1.2.3.4",
          "last_seen_unix": 1718700000,
          "last_seen": "2024-06-18 12:00:00"
        }
      ]
    }
  ]
}
```

---

## 7. HandlerService — inbound 用户管理

写操作（`POST` / `DELETE`）需携带 `Authorization: Bearer <ADMIN_TOKEN>`，详见 [认证](#2-认证)。

### 7.1 GET /api/inbounds/users

列举某 inbound 下的用户。VLESS 账户会解析出 `id`/`flow`，非 VLESS 或无法解析时仅返回 `email`/`level`。

| 参数 | 必填 | 说明 |
|---|---|---|
| `tag` | 是 | inbound tag |

```bash
curl 'http://localhost:8090/api/inbounds/users?tag=vless-in'
```

```json
[
  { "email": "u1", "level": 0, "id": "uuid-1", "flow": "xtls-rprx-vision" }
]
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `email` | string | 用户标识 |
| `level` | int | 用户等级 |
| `id` | string | VLESS UUID（仅 VLESS，省略表示未解析） |
| `flow` | string | VLESS flow（可空） |

### 7.2 GET /api/inbounds/users/count

返回某 inbound 下的用户数。

| 参数 | 必填 | 说明 |
|---|---|---|
| `tag` | 是 | inbound tag |

```bash
curl 'http://localhost:8090/api/inbounds/users/count?tag=vless-in'
```

```json
{ "tag": "vless-in", "count": 3 }
```

### 7.3 POST /api/inbounds/users 🔒

向某 inbound 新增一个 VLESS 用户。请求体为 JSON：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `tag` | string | 是 | inbound tag |
| `email` | string | 是 | 用户标识（同一 inbound 内唯一） |
| `id` | string | 是 | VLESS UUID |
| `flow` | string | 否 | 如 `xtls-rprx-vision`，可空 |
| `level` | int | 否 | 用户等级，默认 0 |

> `encryption` 固定为 `none`（VLESS 要求）。

```bash
curl -X POST http://localhost:8090/api/inbounds/users \
  -H 'Authorization: Bearer <ADMIN_TOKEN>' \
  -H 'Content-Type: application/json' \
  -d '{"tag":"vless-in","email":"u1","id":"b831381d-6324-4d53-ad4f-8cda48b30811","flow":"xtls-rprx-vision"}'
```

成功：

```json
{ "status": "ok", "email": "u1" }
```

### 7.4 DELETE /api/inbounds/users 🔒

按 `email` 从某 inbound 删除用户。

| 参数 | 必填 | 说明 |
|---|---|---|
| `tag` | 是 | inbound tag |
| `email` | 是 | 待删除用户 |

```bash
curl -X DELETE 'http://localhost:8090/api/inbounds/users?tag=vless-in&email=u1' \
  -H 'Authorization: Bearer <ADMIN_TOKEN>'
```

成功：

```json
{ "status": "ok", "email": "u1" }
```

---

## 8. RoutingService — 路由规则（只读）

### 8.1 GET /api/routing/rules

列举当前路由规则（返回快照，不订阅）。无参数。

```bash
curl http://localhost:8090/api/routing/rules
```

```json
[
  { "tag": "direct", "rule_tag": "r1" }
]
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `tag` | string | 规则命中后的出站 tag |
| `rule_tag` | string | 规则自身的 tag（可空） |

---

## 9. 端点 ↔ gRPC 方法对照

| HTTP 端点 | 认证 | gRPC 方法 |
|---|:---:|---|
| `GET /api/sysstats` | | `StatsService.GetSysStats` |
| `GET /api/stats` | | `StatsService.GetStats` |
| `GET /api/stats/online` | | `StatsService.GetStatsOnline` |
| `GET /api/stats/online/iplist` | | `StatsService.GetStatsOnlineIpList` |
| `GET /api/query` | | `StatsService.QueryStats` |
| `GET /api/online/users` | | `StatsService.GetAllOnlineUsers` |
| `GET /api/traffic` | | `StatsService.QueryStats`（聚合） |
| `GET /api/traffic/hourly` | | 后台定时 `StatsService.QueryStats` 差分采样（读本地内存/JSON） |
| `GET /api/online` | | `StatsService.GetAllOnlineUsers` + `GetStatsOnlineIpList`（聚合） |
| `GET /api/all` | | 上述统计方法聚合 |
| `GET /api/inbounds/users` | | `HandlerService.GetInboundUsers` |
| `GET /api/inbounds/users/count` | | `HandlerService.GetInboundUsersCount` |
| `POST /api/inbounds/users` | 🔒 | `HandlerService.AlterInbound`（`AddUserOperation`） |
| `DELETE /api/inbounds/users` | 🔒 | `HandlerService.AlterInbound`（`RemoveUserOperation`） |
| `GET /api/routing/rules` | | `RoutingService.ListRule` |

> 🔒 表示需要 `Authorization: Bearer <ADMIN_TOKEN>`。
