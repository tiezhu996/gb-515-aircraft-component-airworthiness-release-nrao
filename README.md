
# 航空部件适航放行

航空部件、检查任务、证书版本和适航授权协同平台。项目采用前后端分离和明确的领域分层，重点保证状态迁移、RBAC、审计日志、请求追踪与限流在各层保持一致。

## Docker Compose 快速启动

```bash
cp .env.example .env
docker compose up -d --build
docker compose ps
```

- Web 工作台：http://127.0.0.1:18515
- 后端健康检查：http://127.0.0.1:19515/healthz
- 后端 API：http://127.0.0.1:19515/api
- 演示账号统一密码：`Admin123!`（仅限本地演示，生产环境必须更换）

| 用户名 | 角色 | 能力 |
|---|---|---|
| `viewer` | 只读审计员 | 查询实体、证据版本和审计 |
| `operator` | 现场操作员 | 新建、编辑草稿、提交复核；不能批准 |
| `reviewer` | 质量复核员 | 独立发布证书、批准或限制放行 |
| `admin` | 系统管理员 | 复核能力与受控删除权限 |

停止并清理本项目容器与数据卷：

```bash
docker compose down -v --remove-orphans
```

## 主要功能

| 业务模块 | 后端实体 | API 前缀 | 状态流 |
|---|---|---|---|
| 航空部件 | `AircraftPart` | `/api/parts` | received, inspection, hold, released, retired |
| 检查任务 | `InspectionTask` | `/api/inspections` | planned, running, passed, failed |
| 证书记录 | `CertificateRecord` | `/api/certificates` | draft, valid, expired, revoked |
| 放行授权 | `ReleaseAuthorization` | `/api/authorizations` | draft, review, approved, restricted, revoked |

- JWT 登录和 viewer/operator/reviewer/admin 四级 RBAC，数据库角色、Gin middleware、React 路由守卫和按钮权限一致。
- 放行必须经过 `draft -> review -> approved/restricted`，提交者与复核者必须是不同账号，operator 无法自批。
- 证书发布同样要求 reviewer/admin，且发布者不能是当前版本的编制人。
- 证书和授权的每次创建、草稿更新与状态变化都在同一事务写入不可变版本快照和审计日志。
- 所有状态变化使用乐观锁；复核开始后业务字段锁定，防止覆盖已审证据。
- 请求 ID、结构化日志、全局错误映射和 Redis 分布式限流。
- 提供脱敏运行配置、当前会话、审计汇总和单实体审计历史接口。
- 业务工作台支持查询、新建、状态推进、风险标识及操作审计查看。

## 技术栈

| 层次 | 技术 |
|---|---|
| 前端 | React 18 + TypeScript + Vite + Material UI |
| 后端 | Go 1.22 + Gin + GORM |
| 数据 | MySQL + Redis、MinIO |
| 部署 | Docker Compose + Nginx |

## 本地开发

后端可使用 SQLite 开发模式，不需要先启动数据库：

```bash
cd backend
go mod download
DATABASE_DRIVER=sqlite DATABASE_DSN=local.db REDIS_ADDR='' \
JWT_SECRET=local-development-secret PORT=8080 go run ./cmd/server
```

前端开发服务器：

```bash
cd frontend
npm install
npm run dev
```

质量检查：

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
go build ./...

cd ../frontend
npm run typecheck
npm run build

cd ..
docker compose config --quiet
```

也可以从项目根目录执行 `./scripts/validate.sh`。脚本会清理本项目旧卷、从空卷构建启动，检查四角色 RBAC、证书发布、双人放行、不可变版本与审计链，并在结束时关闭容器。设置 `KEEP_RUNNING=1` 可为内置 Browser 验证保留服务。

完整的实际验证记录见 [`VALIDATION.md`](./VALIDATION.md)。

## 目录结构

```text
.
├── backend/
│   ├── cmd/server/                 # 服务入口与优雅退出
│   └── internal/
│       ├── config/                 # 环境配置
│       ├── constants/              # 状态枚举与迁移图
│       ├── database/               # 连接、迁移与演示数据
│       ├── dto/                    # 输入契约
│       ├── handler/                # HTTP 接口
│       ├── middleware/             # JWT、追踪、限流
│       ├── model/                  # GORM 实体
│       ├── repository/             # 持久化边界与版本/审计事务
│       ├── router/                 # 路由装配
│       ├── service/                # 业务规则与审计
│       └── util/                   # 统一 HTTP 响应
├── frontend/src/
│   ├── api/                        # 按实体拆分的 API
│   ├── components/common/          # 共享业务组件
│   ├── hooks/                      # 认证与分页 hooks
│   ├── pages/                      # 五个路由页面
│   ├── router/                     # 路由配置
│   ├── stores/                     # 按实体拆分的状态仓库
│   ├── types/                      # 共享类型与枚举
│   └── utils/                      # 格式化与状态工具
├── docker-compose.yml
└── runtime_smoke.json
```

## 共享枚举位置

| 枚举 | 值 | 前后端出现位置 |
|---|---|---|
| `PartState` | `received, inspection, hold, released, retired` | `backend/internal/constants/status.go`、`frontend/src/types/status.ts` |
| `AuthorizationState` | `draft, review, approved, restricted, revoked` | `backend/internal/constants/status.go`、`frontend/src/types/status.ts` |

每个实体自己的完整迁移图同样位于 `backend/internal/constants/status.go`；页面使用的状态列表位于 `frontend/src/types/status.ts`。修改状态时必须同步两处并更新对应服务测试。

## 环境变量

| 变量 | 说明 |
|---|---|
| `COMPOSE_PROJECT_NAME` | 固定英文 Compose 项目名，支持中文父目录 |
| `DB_NAME/DB_USER/DB_PASSWORD` | 数据库名称与业务账号 |
| `DB_ROOT_PASSWORD` | MySQL 管理员密码（PostgreSQL 项目保留统一模板字段） |
| `JWT_SECRET` | JWT 签名密钥，生产环境必须替换 |
| `FRONTEND_PORT/BACKEND_PORT/DB_PORT` | 宿主机端口映射 |
| `REDIS_PORT` | Redis 宿主机端口 |
| `MINIO_*` | 证据对象存储配置（启用 MinIO 的项目） |

## API 使用示例

```bash
token=$(curl -sS -X POST http://127.0.0.1:19515/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"Admin123!"}' | jq -r '.data.token')

curl -sS http://127.0.0.1:19515/api/overview \
  -H "Authorization: Bearer $token"
```

## License

MIT
