# CPA Usage Keeper

[English README](./README.en.md)

`CPA Usage Keeper` 是一个独立的 CPA 用量持久化与可视化服务。

它依赖 [CLIProxyAPI（CPA）](https://github.com/router-for-me/CLIProxyAPI) 作为后端 CPA 数据来源，目标是在 CPA 之上补充持久化存储与统计分析能力。服务会从 CPA Redis usage 队列消费事件并写入 PostgreSQL 或 SQLite，定时拉取 CPA metadata，暴露聚合 API，并提供内置 Web Dashboard 用于查看 usage、pricing、request health 和 model/API 维度的统计信息。

<p float="left">
  <img src="https://images.bitskyline.com/i/2026/05/govoah.png" width="49%" />
  <img src="https://images.bitskyline.com/i/2026/05/fu4lec.png" width="49%" />
</p>
<p float="left">
  <img src="https://images.bitskyline.com/i/2026/05/fu43px.png" width="49%" />
  <img src="https://images.bitskyline.com/i/2026/05/fu4gh3.png" width="49%" />
</p>

## 功能特性

- 持久保存 CPA usage 数据到 PostgreSQL，兼容 SQLite 单文件模式
- Dashboard 查看请求量、Token、成本、缓存命中率、成功率和延迟
- 支持按时间范围、模型、API Key 和来源筛选用量明细
- 分析页面提供 Token 趋势、模型/API Key/AI Provider 构成和时段热力图
- API Key 独立查询页，可按 CPA API Key 查看专属用量
- 凭证页面展示 Auth File 与 AI Provider 使用情况，支持凭证限额查询与刷新
- 可维护模型价格，用于成本估算和统计展示
- 可选密码登录保护、PostgreSQL Docker/Docker Compose、数据库备份恢复和 systemd 部署

## 快速开始

> 使用前请确认 CPA 配置已开启 usage 统计：`usage-statistics-enabled: true`。

推荐部署路径：

- 第一次部署 CPA + Keeper：优先使用 [Docker Compose](#docker-compose推荐)。
- CPA 已在宿主机运行：使用 [Docker](#dockercpa-已在宿主机运行)。
- 不使用容器：使用 [Linux 二进制](#linux-二进制)。
- 已有 SQLite 数据：如果继续使用 SQLite，无需迁移；如果要切到 PostgreSQL，先按 [从 SQLite 迁移到 PostgreSQL](#从-sqlite-迁移到-postgresql) 做一次性迁移，再切换配置。

公网部署建议启用 `AUTH_ENABLED=true`，并配置 `LOGIN_PASSWORD` 保护数据。

## 部署方式

### Docker Compose（推荐）

仓库提供了一个最简 `docker-compose.example.yml` 示例，用于同时部署 PostgreSQL 和 CPA Usage Keeper；如果 CPA 也需要容器化，可把 CPA 服务加入同一个 Compose 网络：

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-cpa_usage_keeper}
      POSTGRES_USER: ${POSTGRES_USER:-keeper}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD}
      TZ: ${TZ:-Asia/Shanghai}
    ports:
      - "127.0.0.1:5432:5432"
    volumes:
      - postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U $${POSTGRES_USER} -d $${POSTGRES_DB}"]
      interval: 10s
      timeout: 5s
      retries: 5
    restart: unless-stopped

  cpa-usage-keeper:
    image: ghcr.io/shay-wong/cpa-usage-keeper:latest
    ports:
      - "8080:8080"
    environment:
      CPA_BASE_URL: ${CPA_BASE_URL}
      CPA_MANAGEMENT_KEY: ${CPA_MANAGEMENT_KEY}
      DATABASE_DRIVER: ${DATABASE_DRIVER:-postgres}
      DATABASE_URL: ${DATABASE_URL:?set DATABASE_URL}
      AUTH_ENABLED: ${AUTH_ENABLED:-false}
      LOGIN_PASSWORD: ${LOGIN_PASSWORD:-}
      TZ: ${TZ:-Asia/Shanghai}
    volumes:
      - ./data:/data
    depends_on:
      postgres:
        condition: service_healthy
    restart: unless-stopped

volumes:
  postgres-data:
```

如果想用 `.env` 文件管理 Keeper 配置，可以把上面 `cpa-usage-keeper` 的 `environment` 改成 `env_file`：

```yaml
    env_file:
      - .env
```

然后在宿主机的 `docker-compose.yml` 同一目录创建 `.env` 文件，例如：

```env
TZ=Asia/Shanghai
CPA_BASE_URL=http://cli-proxy-api:8317
CPA_MANAGEMENT_KEY=replace-with-your-management-key
REDIS_QUEUE_ADDR=cli-proxy-api:8317
AUTH_ENABLED=true
LOGIN_PASSWORD=replace-with-your-login-password
```

`env_file` 中的路径相对 `docker-compose.yml` 所在目录解析；上面的 `.env` 会被注入 Keeper 容器，效果等同于为容器设置这些环境变量。

启动：

```bash
docker compose up -d
```

停止：

```bash
docker compose down
```

PostgreSQL 数据保存在 Docker named volume `postgres-data` 中，Keeper 日志写入 `./data`。示例把 PostgreSQL 端口绑定到宿主机 `127.0.0.1:5432`，可在 macOS 上用 TablePlus、psql 等工具连接 live DB：数据库 `cpa_usage_keeper`，用户 `keeper`，密码使用 `.env` 中的 `POSTGRES_PASSWORD`。

### 从 SQLite 迁移到 PostgreSQL

全新 PostgreSQL 部署会在首次启动时自动建表并标记迁移版本；但已有 SQLite 数据不会在切换 `DATABASE_DRIVER=postgres` 时自动导入。SQLite 模式仍然受支持，如果你选择继续使用 SQLite，保持 `DATABASE_DRIVER=sqlite` 或同时留空 `DATABASE_DRIVER` 和 `DATABASE_URL` 即可，不需要执行本节迁移。自动导入可能误迁损坏的 SQLite、覆盖已写入的 PostgreSQL 数据，或让回滚边界不清楚，所以要切到 PostgreSQL 的老用户需要按下面步骤手动做一次性迁移。

1. 停止正在写入 SQLite 的 Keeper 服务，避免 `app.db-wal` 里还有未 checkpoint 的数据：

```bash
docker compose stop cpa-usage-keeper
```

如果不是 Docker Compose 部署，请用你当前的进程管理方式停止 Keeper。

2. 备份 SQLite 数据库目录，至少保留 `app.db`、`app.db-wal` 和 `app.db-shm`：

```bash
cp -a /path/to/keeper-data /path/to/keeper-data.backup-before-postgres
```

3. 启动 PostgreSQL，并确认目标库为空。Docker Compose 部署可直接使用本 README 上面的 `postgres` 服务示例。

4. 在有 Go 1.22+ 的机器上运行迁移工具。迁移命令从宿主机执行时，`DATABASE_URL` 通常要使用 PostgreSQL 暴露到宿主机的地址，例如 `127.0.0.1:5432`；Keeper 容器内运行时才使用 Compose 服务名 `postgres:5432`。

```bash
go run ./cmd/migrate-sqlite-to-postgres \
  -sqlite /path/to/keeper-data/app.db \
  -database-url 'postgres://keeper:replace-with-password@127.0.0.1:5432/cpa_usage_keeper?sslmode=disable'
```

默认情况下，迁移工具要求目标表为空；如果目标库里已经有失败迁移留下的数据，先确认要完全替换后再加 `-truncate`：

```bash
go run ./cmd/migrate-sqlite-to-postgres \
  -sqlite /path/to/keeper-data/app.db \
  -database-url 'postgres://keeper:replace-with-password@127.0.0.1:5432/cpa_usage_keeper?sslmode=disable' \
  -truncate
```

5. 迁移成功后，把 Keeper 配置切到 PostgreSQL。Docker Compose 中的 `DATABASE_URL` 应使用容器网络地址：

```env
DATABASE_DRIVER=postgres
DATABASE_URL=postgres://keeper:replace-with-password@postgres:5432/cpa_usage_keeper?sslmode=disable
```

6. 重启 Keeper 并验证服务和数据：

```bash
docker compose up -d cpa-usage-keeper
curl -f http://127.0.0.1:8080/healthz
```

也可以对比迁移前后的核心表行数：

```bash
sqlite3 /path/to/keeper-data/app.db 'select count(*) from usage_events;'
psql 'postgres://keeper:replace-with-password@127.0.0.1:5432/cpa_usage_keeper?sslmode=disable' -c 'select count(*) from usage_events;'
```

如果 SQLite 已损坏，先不要直接迁移 live `app.db`；请优先使用最近一次健康备份，或先用 SQLite 官方工具恢复到一个 `PRAGMA quick_check` 返回 `ok` 的临时库，再把临时库作为 `-sqlite` 输入。回滚时停止 Keeper，把配置切回 SQLite，并恢复第 2 步备份的数据目录。

### Docker（CPA 已在宿主机运行）

复制配置模板并编辑，至少设置 `CPA_BASE_URL`、`CPA_MANAGEMENT_KEY`、`REDIS_QUEUE_ADDR`、`AUTH_ENABLED` 和 `LOGIN_PASSWORD`：

```bash
cp .env.example .env
vim .env
```

宿主机运行 CPA 时，`.env` 中通常需要这样设置：

```env
CPA_BASE_URL=http://host.docker.internal:8317
CPA_MANAGEMENT_KEY=replace-with-your-management-key
REDIS_QUEUE_ADDR=host.docker.internal:8317
AUTH_ENABLED=true
LOGIN_PASSWORD=replace-with-your-login-password
```

```bash
docker run -d \
  --name cpa-usage-keeper \
  --add-host=host.docker.internal:host-gateway \
  -p 8080:8080 \
  -v "$(pwd)/keeper:/data" \
  --env-file .env \
  ghcr.io/shay-wong/cpa-usage-keeper:latest
```

### Linux 二进制

#### 下载

在 [Releases](https://github.com/shay-wong/cpa-usage-keeper/releases/latest) 下载对应架构的 Linux 二进制包，或使用命令行下载：

```bash
curl -L -o cpa-usage-keeper.tar.gz "<替换为 Linux 二进制包下载地址>"
mkdir -p cpa-usage-keeper
tar -xzf cpa-usage-keeper.tar.gz -C cpa-usage-keeper --strip-components=1
cd cpa-usage-keeper
```

请在 Releases 页面复制 `linux_amd64` 或 `linux_arm64` 包的下载地址，并替换上面命令中的占位符。

#### 配置和运行

复制配置模板并编辑，具体配置项参考 [配置](#配置)：

```bash
cp .env.example .env
vim .env
./cpa-usage-keeper
```

#### systemd 常驻运行

Linux 二进制包内置 `cpa-usage-keeper.service`，可直接注册为 `systemd` 服务。启动后进程由 systemd 托管，关闭 SSH 或终端不会结束进程。

`systemd` 的 `WorkingDirectory` 需要绝对路径。下面的 `sed` 命令会把当前目录自动写入 service 文件：

```bash
sudo cp cpa-usage-keeper.service /etc/systemd/system/cpa-usage-keeper.service # 复制 service 文件到 systemd 目录
sudo sed -i "s|__CPA_USAGE_KEEPER_DIR__|$(pwd)|g" /etc/systemd/system/cpa-usage-keeper.service # 写入当前目录作为 WorkingDirectory
sudo systemctl daemon-reload # 重新加载 systemd 配置
sudo systemctl enable --now cpa-usage-keeper # 设置开机自启并立即启动服务
```

常用命令：

```bash
sudo systemctl status cpa-usage-keeper # 查看服务状态
sudo journalctl -u cpa-usage-keeper -f # 实时查看服务日志
sudo systemctl restart cpa-usage-keeper # 重启服务
```

## 配置

复制配置模板：

```bash
cp .env.example .env
```

新手部署时优先看“最小必填”和“Web 访问与反代”两组，其它配置保持默认即可。

### 最小必填

| 变量 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `CPA_BASE_URL` | 是 | - | Keeper 服务端访问 CPA 的地址。Docker Compose 内通常是 `http://cli-proxy-api:8317`，可以是内网地址或容器服务名 |
| `CPA_MANAGEMENT_KEY` | 是 | - | CPA management key，用于读取 CPA 管理接口数据 |

### Web 访问与反代

| 变量 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `APP_PORT` | 否 | `8080` | Keeper HTTP 监听端口 |
| `APP_BASE_PATH` | 否 | 根路径 | Keeper 子路径部署前缀，例如 `/keeper`；留空表示部署在 `/` |
| `CPA_PUBLIC_URL` | 否 | `CPA_BASE_URL` | 浏览器访问 CPA 的公开地址，用于“返回 CPA”跳转 |

`APP_BASE_PATH` 必须为空或以 `/` 开头；例如 `/cpa`，`/cpa/` 会规范为 `/cpa`。

`CPA_PUBLIC_URL` 可填写域名、带协议的完整地址或相对路径，例如 `https://cpa.example.com`、`https://cpa.example.com/cpa/` 或 `/cpa/`。前端会自动追加 `management.html`，并兼容末尾已有 `/` 或已经填写到 `management.html` 的情况。未配置时，Keeper 会把 `CPA_BASE_URL` 作为跳转基准；如果它是 Docker service、`0.0.0.0`、`localhost` 或无点号的内部主机名，前端会自动替换成当前浏览器访问 Keeper 的主机名。

`CPA_BASE_URL` 用于服务端访问 CPA，可以是 Docker 内部服务名或内网地址。只有自动转换后的浏览器跳转地址不符合你的公开访问方式时，才需要额外设置 `CPA_PUBLIC_URL`。

### 登录保护

| 变量 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `AUTH_ENABLED` | 否 | `false` | 是否启用登录保护 |
| `LOGIN_PASSWORD` | 鉴权启用时必填 | - | 登录密码 |
| `AUTH_SESSION_TTL` | 否 | `168h` | 登录 session 有效时长 |

### 时区与请求行为

| 变量 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `TZ` | 否 | `Asia/Shanghai` | 统计和展示使用的时区；Today、按天统计、页面时间、日志时间和每日清理时间都会按这个时区计算 |
| `REQUEST_TIMEOUT` | 否 | `30s` | 请求 CPA HTTP 接口和 Redis 队列的超时时间 |
| `TLS_SKIP_VERIFY` | 否 | `false` | 跳过 CPA HTTPS 和 Redis 队列 TLS 的证书验证；仅在使用自签名证书时启用 |

### Auth Files 限额刷新

| 变量 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `QUOTA_AUTO_REFRESH_ENABLED` | 否 | `false` | 是否启用 Auth Files 限额自动刷新；仅在后台页面可见并持续心跳时执行 |
| `QUOTA_AUTO_REFRESH_INTERVAL` | 否 | `5m` | Auth Files 限额自动刷新间隔，最低 `60s`，仅在后台页面活跃时生效 |
| `QUOTA_REFRESH_WORKER_LIMIT` | 否 | `10` | Auth Files 限额刷新队列最大并发数，最大 `100` |

### Redis 队列高级配置

| 变量 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `REDIS_QUEUE_ADDR` | 否 | `CPA_BASE_URL` 主机名 + `8317` | CPA Redis/RESP TCP 地址；一般保持空即可。非默认端口或单独暴露 Redis stream 时填写 `host:port` |
| `REDIS_QUEUE_TLS` | 否 | `false` | 是否使用 TLS 连接 Redis 队列；显式设置 `REDIS_QUEUE_ADDR` 且需要 TLS 时设为 `true` |
| `REDIS_QUEUE_BATCH_SIZE` | 否 | `10000` | 每次最多拉取的队列记录数 |
| `REDIS_QUEUE_IDLE_INTERVAL` | 否 | `1s` | 队列为空时的检查间隔 |

### 存储、日志与备份

| 变量 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `DATABASE_DRIVER` | 否 | `sqlite` | 主数据库类型；推荐 PostgreSQL，设为 `postgres`。继续使用 SQLite 时保持 `sqlite` 或同时留空 `DATABASE_DRIVER` 和 `DATABASE_URL`；设置 `DATABASE_URL` 且未显式设置 driver 时默认使用 PostgreSQL |
| `DATABASE_URL` | PostgreSQL 模式必填 | - | PostgreSQL 连接串；Docker Compose 中通常使用 `postgres` 服务名，SQLite 模式留空 |
| `WORK_DIR` | 否 | `./data` | 应用工作目录；日志和 SQLite 兼容模式下的数据库/备份默认写入这里，PostgreSQL 数据保存在 PostgreSQL 实例或 Docker volume 中 |
| `LOG_LEVEL` | 否 | `info` | 日志级别 |
| `LOG_FILE_ENABLED` | 否 | `true` | 是否写入持久化日志文件 |
| `LOG_RETENTION_DAYS` | 否 | `7` | 日志保留天数；`0` 表示不自动清理 |
| `BACKUP_ENABLED` | 否 | `false` | 是否启用内置数据库备份；SQLite 使用 `.db` 文件备份，PostgreSQL 使用 `pg_dump`/`pg_restore` 自定义格式备份 |
| `BACKUP_INTERVAL` | 否 | `24h` | 自动数据库备份间隔 |
| `BACKUP_RETENTION_DAYS` | 否 | `7` | 自动数据库备份保留天数 |

### 内置 HTTPS

| 变量 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `TLS_ENABLED` | 否 | `false` | 是否让 Keeper 自己启用 HTTPS/TLS |
| `TLS_CERT_FILE` | 启用 TLS 时必填 | - | HTTPS 证书文件路径 |
| `TLS_KEY_FILE` | 启用 TLS 时必填 | - | HTTPS 私钥文件路径 |

通常建议在 nginx、Caddy 等反向代理层处理 HTTPS。只有需要 Keeper 进程直接提供 HTTPS 时，才设置 `TLS_ENABLED=true`，并填写 `TLS_CERT_FILE` 和 `TLS_KEY_FILE`；相对路径会按 `.env` 所在目录解析。

安全与数据说明：

- 存储页可查看数据库和备份占用，配置请求日志/用量日志清理范围、备份数据范围、备份时间、最大备份数，并按数据范围恢复备份。
- 备份会保存应用数据库中的原始数据，备份文件不做加密；SQLite 模式生成 `.db` 文件，PostgreSQL 模式生成 `pg_dump -Fc` 的 `.dump` 文件，运行环境必须包含 `pg_dump`、`pg_restore` 和 `psql`。
- 面向浏览器的 API 会对 key-like source/lookup 字段做脱敏或稳定公开标识映射，但不会修改数据库原始值。
- 公开部署建议开启 `AUTH_ENABLED=true`，并在反向代理层配置 HTTPS。
- 登录 session 存在服务进程内存中，服务重启后已登录 session 会失效。
- Redis inbox 原始消息会自动清理：成功数据保留到当天结束后清理，失败数据保留 7 天。

## Nginx反代

部署到 `/cpa` 时设置 `APP_BASE_PATH=/cpa`，并在反向代理中保留该前缀：

```nginx
location /cpa/ {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
}
```

如果 CPA 管理页和 Keeper 使用同一个浏览器域名，可以不配置 `CPA_PUBLIC_URL`。例如 Docker Compose 中 `CPA_BASE_URL=http://cli-proxy-api:8317`，用户通过 `http://192.168.1.23:8080` 访问 Keeper 时，“返回 CPA”会自动跳转到 `http://192.168.1.23:8317/management.html`。

如果 CPA 管理页在其它域名、端口或路径下，请配置 `CPA_PUBLIC_URL`，例如：

```env
CPA_PUBLIC_URL=https://cpa.example.com
```

## 项目结构

```text
cmd/server/              应用入口
internal/api/            HTTP 路由与处理器
internal/app/            应用装配与启动
internal/auth/           内存 session 鉴权
internal/backup/         SQLite/PostgreSQL 数据库备份恢复管理
internal/benchmark/      聚合性能基准测试辅助
internal/config/         环境配置加载
internal/cpa/            CPA 客户端与类型定义
internal/entities/       GORM 数据模型
internal/helper/         后端通用辅助方法
internal/logging/        日志初始化与保留策略
internal/poller/         后台队列消费与 metadata 同步
internal/quota/          quota 缓存、刷新与查询服务
internal/redact/         前端展示字段脱敏
internal/repository/     数据库访问、迁移与聚合逻辑
internal/service/        usage、pricing 与身份数据服务
internal/timeutil/       项目时区与时间工具
internal/updatecheck/    GitHub Release 更新检查
internal/version/        构建版本信息
deploy/linux/            Linux systemd 服务文件
web/                     React + TypeScript 前端
```

## 本地开发

### 前置依赖

- Go 1.22+
- Node.js 22+
- npm
- 已运行的 [CLIProxyAPI（CPA）](https://github.com/router-for-me/CLIProxyAPI)

### 本地启动

1. 复制并编辑本地配置，至少设置 `CPA_BASE_URL` 和 `CPA_MANAGEMENT_KEY`。如果 CPA 的 Redis/RESP 端口不是默认 `8317`，同时设置 `REDIS_QUEUE_ADDR`：

```bash
cp .env.example .env
vim .env
```

2. 启动后端：

```bash
go run ./cmd/server/main.go
```

3. 在另一个终端安装前端依赖并启动开发服务器：

```bash
npm --prefix ./web ci
npm --prefix ./web run dev -- --host 127.0.0.1
```

前端开发服务器默认把 `/api` 代理到 `http://127.0.0.1:8080`，访问 `http://127.0.0.1:5173` 即可联调。如果后端使用了其他端口：

```bash
VITE_API_PROXY_TARGET=http://127.0.0.1:9090 npm --prefix ./web run dev -- --host 127.0.0.1
```

### 测试

运行完整的本地验证基线：

```bash
make verify
```

也可以单独运行各项检查：

```bash
go test ./cmd/... ./internal/...
npm --prefix ./web run test
npm --prefix ./web run lint
npm --prefix ./web run typecheck
npm --prefix ./web run build
```

## Star History

<p>
  <img src="https://api.star-history.com/chart?repos=willxup/cpa-usage-keeper&type=date&legend=top-left" />
</p>


## License

本项目基于 [MIT License](./LICENSE) 开源。
