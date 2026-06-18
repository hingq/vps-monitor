# VPS Stats API 文档

本服务连接 Xray 的 gRPC `StatsService`（`github.com/xtls/xray-core/app/stats/command`），
把其全部 6 个 gRPC 方法一一映射为 HTTP API，并额外提供若干业务聚合端点。

所有响应均为 `application/json; charset=utf-8`，采用缩进美化输出。
查询出错时返回 HTTP `500`，响应体为 `{"error": "..."}`；
单计数器端点缺少必填参数时返回 HTTP `400`。

## 配置（环境变量）

| 变量 | 说明 | 默认值 |
|---|---|---|
| `XRAY_API_ADDR` | Xray gRPC API 服务端地址 | `67.209.178.119:80` |
| `WEB_LISTEN` | 本服务 HTTP 监听地址 | `:8090` |

启动：`go run ./cmd/vps`，或编译后 `./vps`。

## 公共参数说明

- `reset`（布尔，可选）：是否在读取后重置计数器。仅 `true` 或 `1` 视为真，其余（含缺省）视为假。
- `name`（字符串）：计数器/用户名称，单计数器端点必填。
- `pattern`（字符串，可选）：计数器名称匹配前缀，空表示匹配全部。

---

## 一、gRPC 方法 1:1 映射端点

### 1. GET /api/sysstats — `GetSysStats`

系统运行状态。无参数。

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

### 2. GET /api/stats — `GetStats`

查询单个计数器取值。

查询参数：`name`（必填）、`reset`（可选）。

`GET /api/stats?name=user>>>foo>>>traffic>>>uplink`

```json
{
  "name": "user>>>foo>>>traffic>>>uplink",
  "value": 10485760,
  "value_human": "10.00 MB"
}
```

### 3. GET /api/stats/online — `GetStatsOnline`

查询单个计数器的在线数（如用户在线连接/IP 计数）。

查询参数：`name`（必填）、`reset`（可选）。

`GET /api/stats/online?name=user>>>foo>>>online`

```json
{
  "name": "user>>>foo>>>online",
  "value": 3
}
```

### 4. GET /api/stats/online/iplist — `GetStatsOnlineIpList`

查询单个用户的在线来源 IP 及最后活跃时间，按 IP 排序。

查询参数：`name`（必填，用户名）。

`GET /api/stats/online/iplist?name=foo`

```json
[
  {
    "ip": "1.2.3.4",
    "last_seen_unix": 1718700000,
    "last_seen": "2024-06-18 12:00:00"
  }
]
```

### 5. GET /api/query — `QueryStats`

返回原始（未聚合）计数器列表。

查询参数：`pattern`（可选）、`reset`（可选）。

`GET /api/query?pattern=user>>>`

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

### 6. GET /api/online/users — `GetAllOnlineUsers`

返回排序后的在线用户名列表。无参数。

```json
["alice", "bob", "foo"]
```

---

## 二、业务聚合端点

### 7. GET /api/traffic

基于 `QueryStats`，把名称形如 `category>>>name>>>traffic>>>direction` 的计数器
按 `category -> name` 聚合上下行，分类固定顺序为 user / inbound / outbound。

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

### 8. GET /api/online

基于 `GetAllOnlineUsers` + 每用户 `GetStatsOnlineIpList`，返回在线用户及其来源 IP。
单个用户的 IP 查询失败不影响整体，其 `ips` 置空。

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

### 9. GET /api/all

`sysstats` + `traffic` + `online` 三合一。

```json
{
  "sys_stats": { "uptime_seconds": 123456, "...": "..." },
  "traffic": [ { "category": "user", "...": "..." } ],
  "online": [ { "user": "foo", "...": "..." } ]
}
```

---

## 端点 ↔ gRPC 方法对照

| HTTP 端点 | gRPC 方法 |
|---|---|
| `GET /api/sysstats` | `GetSysStats` |
| `GET /api/stats` | `GetStats` |
| `GET /api/stats/online` | `GetStatsOnline` |
| `GET /api/stats/online/iplist` | `GetStatsOnlineIpList` |
| `GET /api/query` | `QueryStats` |
| `GET /api/online/users` | `GetAllOnlineUsers` |
| `GET /api/traffic` | `QueryStats`（聚合） |
| `GET /api/online` | `GetAllOnlineUsers` + `GetStatsOnlineIpList`（聚合） |
| `GET /api/all` | `GetSysStats` + `QueryStats` + `GetAllOnlineUsers` + `GetStatsOnlineIpList`（聚合） |
