# banana-mw

`banana-mw` 是一个 Docker Compose 启动的接口保护中间件，面向 OTA 配置下发接口。它基于 go-chi v5 和标准 `net/http`，负责路径准入、token 校验、响应缓存、48 小时访问统计和内嵌管理面板。

## Docker 启动

上传整个目录到服务器后，准备运行时配置并启动：

```sh
cp config.example.yaml data/config.yaml
# 编辑 data/config.yaml，至少修改 upstream
```

然后执行：

```sh
docker compose build
docker compose up -d
```

默认 Compose 会：

- 构建当前目录的 Go 程序。
- 启动单个 `banana-mw` 容器（host 网络）。
- 反向代理监听 `port`（默认 `8080`）；`is_admin=1` 时管理面板监听独立的 `admin_port`（默认 `8081`）。
- 将 `./data` 只读挂载为容器内 `/app/data`（配置为 `data/config.yaml`）。
- 提供 `host.docker.internal` 到宿主机的映射，方便容器访问宿主机 Redis。

查看日志：

```sh
docker compose logs -f
```

停止服务：

```sh
docker compose down
```

## 配置

运行时配置文件是 `data/config.yaml`（YAML）。程序会优先读取 `data/config.yaml`，再回退根目录下的 `config.yaml` 或 `config.json`（兼容旧部署）。

`data/` 目录不纳入 Git，首次使用请从 `config.example.yaml` 复制，详见 `data/README.md`。

`upstream: https://example.com` 是占位值，启动前必须改成真实上游地址，否则程序会直接退出。

关键字段：

- `upstream`：真实上游地址，必须配置。
- `allowed_prefixes`：允许代理的路径前缀，其他路径直接拦截。
- `cache_ttl_200`：200 响应 body/header 缓存时间。
- `cache_ttl_4xx`：4xx token/path 失败结果缓存时间。
- `cache_include_query`：默认 `false`，缓存 key 仍按 method、path、User-Agent、token 生成；为 `true` 时把 query 参数也纳入缓存 key，但会排除 `token` 和 `s`。
- `port`：反向代理监听端口，默认 `8080`。如果修改它，也要同步修改 `docker-compose.yml` 的端口映射。
- `admin_port`：管理面板监听端口，默认 `8081`，仅 `is_admin=1` 时启用，必须与 `port` 不同（否则启动报错）。代理与管理分属两个独立 HTTP server，管理路由不再出现在 `port` 上。
- `log_level`：`info`（默认）、`debug`（每条代理请求输出排障日志）、`warn`、`error`。
- `secure`：为 `1` 时启用 Redis token 校验；Redis 中没有登记的 token 返回 401，不进入缓存代理阶段。
- `secure_redis_key_prefix`：secure token 在 Redis 中的 key 前缀（后接 token）；留空默认为 `secure_session::user::`（须与上游/面板写入 Redis 时使用的前缀一致）。
- `token_redis_addr`：Redis 地址。Redis 在宿主机上时使用 `host.docker.internal:6379`。
- `redis_dial_timeout`、`redis_read_timeout`、`redis_write_timeout`：Redis 连接和读写超时，默认分别为 `3s`、`1s`、`1s`。
- `stats_enabled`：为 `1` 时启用 48 小时访问统计。
- `stats_prefix`：统计路径前缀，支持具体前缀或 `*`。
- `stats_queue_size`：统计写入队列大小，默认 `10000`。队列满时丢弃统计事件，不阻塞业务请求。
- `stats_workers`：统计写 Redis 的后台 worker 数，默认 `2`。
- `stats_write_timeout`：单次统计写 Redis 超时，默认 `1s`。
- `stats_clean_inactive_time`：低频整 key 清理的不活跃阈值，默认按 48 小时窗口处理。
- `stats_clean_interval_hours`：stats 后台清理间隔，未配置时默认 1 小时。
- `is_admin`：为 `1` 时在 `admin_port` 上额外启动管理接口/页面（与 `port` 上的反向代理是两个独立的 HTTP server）；为 `0` 时完全不暴露管理入口。
- `admin_token`：管理接口 Bearer token。为空时管理 API 返回 503，视为未启用。
- `read_header_timeout`、`read_timeout`、`write_timeout`、`idle_timeout`、`shutdown_timeout`：HTTP server 超时和优雅退出等待时间，默认分别为 `5s`、`30s`、`0s`、`120s`、`10s`。

`config.example.yaml` 是带注释的通用模板；实际运行配置放在 `data/config.yaml`。

## 管理面板

面板页面和接口内嵌在同一个二进制里：

- `GET /_gogoadmin/`：管理页面（纯监控看板，支持自动刷新、过滤、token 掩码、48h 趋势图）。
- `GET /_gogoadmin/api/stats`：面板 JSON 数据，包含 path 和 token 两个维度的 48 小时统计。每条记录含 `count`（累计）、`count_48h`、`ips`、`uas`、`hourly`（最近 48 小时逐小时访问次数，数组末位为当前小时）。
- `GET /_gogoadmin/api/health`：服务健康度，包含统计队列水位（`length`/`capacity`）、丢弃事件数（`dropped`）、worker 数、Redis 连通性与延迟、uptime、版本号。
- `GET /banana-mw/api/get`：兼容旧 stats 查询接口，只返回 path 维度。

所有管理 API 都需要请求头 `Authorization: Bearer <admin_token>`。未配置 `admin_token` 时接口返回 503；token 错误时返回 401（并记录 IP/UA 告警日志）；`/stats` 在 `stats_enabled` 未启用时返回 503，`/health` 始终可用。管理页面与 API 响应带 `Cache-Control: no-store`、`X-Content-Type-Options: nosniff`、`X-Frame-Options: DENY` 和 CSP（`frame-ancestors 'none'`）。

管理面板由独立的 HTTP server 监听 `admin_port`（默认 `8081`），与 `port` 上的反向代理完全分离：`port` 上不再注册任何 `/_gogoadmin/`、`/banana-mw/api/get` 路由，避免路径冲突，也让管理入口可单独做网络隔离。请在生产环境只对内网放行 `admin_port`。
Stats 的 IP、UA、token、path 明细以 Redis sorted set 保存（score 为最后访问时间）；访问次数按小时分桶记录在 `::hours` hash 中（field 为小时桶 `unix/3600`），`count_48h` 与趋势图都由这些小时桶汇总，桶字段数恒定有界（约 49 个）。后台 worker 异步写入 Redis，清理任务定期裁剪 48 小时前的明细并删除过期小时桶。

## 架构

启动入口在 `cmd/banana-mw/main.go`，业务逻辑在 `internal/server` 与各子包中：

1. 读取 `data/config.yaml`（容器内为 `/app/data/config.yaml`）。
2. 创建 `httputil.NewSingleHostReverseProxy` 指向 `upstream`。
3. 初始化 200 响应缓存和 4xx 响应缓存。
4. 当 `secure=1`、`stats_enabled=1` 或 `is_admin=1` 时初始化 Redis。
5. 启动 stats worker 和 stats cleaner，并绑定进程退出信号。
6. 在 `port` 上启动反向代理 HTTP server，按中间件链处理业务请求。
7. 当 `is_admin=1` 时，再在 `admin_port` 上启动一个独立的管理 HTTP server。
8. 收到退出信号后，对两个 server 按 `shutdown_timeout` 做优雅退出。

代理路由（`port`）在 `internal/server` 中组装，顺序是：

1. panic recovery。
2. HTTP method 限制，只允许 GET 和 POST。
3. User-Agent 必须非空。
4. 路径必须匹配 `allowed_prefixes`。
5. token 必须来自 query/post form，或匹配 `/link/{token}`。
6. 可选 Redis secure token 校验。
7. 可选 stats 统计。
8. 先查 4xx 缓存，再查 200 缓存。
9. 未命中缓存时反向代理到上游，并按响应状态写入缓存。

管理路由（`admin_port`）只挂 panic recovery 与管理处理器（`/_gogoadmin/`、`/banana-mw/api/get`），不经过上述代理 guard 与缓存中间件。

## 本地开发

```sh
make run
make test
make build
make fmt
```

本地直接运行时也优先读取当前目录下的 `data/config.yaml`。
本机验证建议使用 `go test ./...`、`go build ./...` 和 `docker compose config`；实际服务器部署再执行 `docker compose build`。
