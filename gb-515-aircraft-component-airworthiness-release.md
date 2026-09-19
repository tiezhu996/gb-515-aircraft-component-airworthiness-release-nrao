请生成 `aircraft-component-airworthiness-release`「航空部件适航放行」Go 全栈项目，面向航空维修制造企业管理部件、检查项、证书版本和适航放行决定。不要实现航班售票、预约、订单、仓库或财务业务。

## 项目主要需求

复杂度下限：核心实体不少于 3 个、核心页面不少于 4 个、横切关注点不少于 2 个、共享前端组件不少于 3 个、自定义 hooks/utils 不少于 2 个、后端中间件不少于 2 个。

### 核心实体

`AircraftPart`（部件与序列号）、`InspectionTask`（检查项和结果）、`CertificateRecord`（证书版本）、`ReleaseAuthorization`（适航放行与限制）全链路贯穿数据库、Go 分层和前端。

### 核心页面

`/parts` 部件；`/inspections` 检查；`/certificates` 证书；`/authorizations` 放行授权；`/audit` 审计。`PartStatusBadge` 在部件和授权页共用，`CertificatePanel` 在证书和检查页共用。

### 横切关注点

RBAC 与双人复核同步 DB 角色、Go middleware、路由守卫、前端按钮；证书和授权版本必须写审计、操作者、请求 ID；全局错误处理和限流跨层实现。

### 共享枚举/组件

同步 `PartState`（received/inspection/hold/released/retired）与 `AuthorizationState`（draft/review/approved/restricted/revoked）。共享 `StatusBadge`、`CertificatePanel`、`ConfirmDialog`，hooks 为 `useAuth`、`usePagination`。

### 技术与规模要求

前端 React 18 + TypeScript + Vite + Material UI；后端 Go 1.22 + Gin + GORM；MySQL + Redis + MinIO。目标 2800–4000 行、28–40 个 `.go` 文件。

### 文件结构强制清单

前端必须有 `api/stores/types/components/common/hooks/pages/router/utils`；后端必须有 `model/dto/repository/service/handler/router/middleware/constants/util`，禁止职责合并。

### 结构红线

严禁合并职责到单一文件；部件、检查、证书和授权各自保持独立模块。

### 部署与交付

根目录必须提供 `docker-compose.yml`（顶层 `name: aircraft-component-airworthiness-release`，且不写 `version:`）、`.env` 和 `.env.example`（均含 `COMPOSE_PROJECT_NAME=aircraft-component-airworthiness-release`）、`README.md`、`frontend/Dockerfile`、`backend/Dockerfile` 和 `frontend/nginx.conf`。前端端口 `18515`、后端端口 `19515`；Nginx `/api` 反代、数据库健康检查、命名卷和 `condition: service_healthy` 齐全，提供真实 `/healthz`、Git 初始化。
