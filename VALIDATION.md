# 验证记录

## 2026-09-19 放行前证据闸门

本次变更为放行授权新增放行前证据闸门，以下命令均实际执行成功：

```bash
cd backend
gofmt -w <本次修改的 Go 文件>
go test ./...
go test -race ./...
go vet ./...
go build ./...

cd ../frontend
npm install --no-audit --no-fund
npm run typecheck
npm run build
```

- 非测试 Go 代码：3392 行；非测试 `.go` 文件：39 个。
- 新增服务层测试覆盖：提交时证据缺失/状态不符被逐条阻断且草稿保持 v1；批准时重新读取证据、等待期间证据漂移被阻断、恢复后放行成功；证据失效不影响 review→draft 与 review→restricted；并发批准仅一次成功、重复批准被拒绝；既有双人复核与证书异人发布回归全部通过。
- 使用 SQLite 开发模式启动真实后端（`DATABASE_DRIVER=sqlite`）并通过 curl 走通完整 HTTP 流程：无证据提交返回 HTTP 422 `evidence_gate_blocked` 及三条阻断项且草稿保持 v1；补齐部件 hold、检查 passed、证书 valid 后提交成功；等待期间部件离开 hold 时批准返回 422 且保持 review v2；恢复后批准为 approved v3；重复批准返回 422；approved→restricted 不受影响。
- `scripts/validate.sh` 已同步扩展：空证据提交拒绝、证据链搭建（部件 received→inspection→hold、检查 planned→running→passed、证书异人发布 valid）、等待期证据漂移拒绝、重复批准拒绝等断言均已加入；本环境无 Docker，Compose 段未在本机重跑，脚本通过 `sh -n` 语法检查。
- 前端 `npm run typecheck` 与 `npm run build` 通过；阻断项经 API 客户端 `ApiError.blockers` 传入实体仓库并在页面以列表形式展示。

## 2026-08-22 基线验证

### 代码质量

以下命令均实际执行成功：

```bash
cd backend
gofmt -w <本次修改的 Go 文件>
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

- 非测试 Go 代码：3239 行。
- 非测试 `.go` 文件：38 个。
- 服务层回归测试覆盖证书异人发布、放行双人复核、operator 越权、同人伪装 reviewer、复核后编辑锁定、版本与审计数量。

### 空卷 Compose 与 API

执行 `KEEP_RUNNING=1 ./scripts/validate.sh`，脚本先运行 `docker compose down -v --remove-orphans`，再从空命名卷构建并启动 MySQL、Redis、MinIO、backend、frontend。验证结果：

- `/healthz` 返回 database=`ready`、redis=`ready`。
- `viewer` 可读取部件与会话，POST 写入返回 HTTP 403。
- `operator` 创建放行草稿 v1，并提交为 review v2；其自批请求返回 HTTP 403。
- `reviewer` 独立批准为 v3，`submittedBy=operator`、`reviewedBy=reviewer`。
- 三个放行版本分别保留 actor、request ID、状态、证据和原因。
- `operator` 创建证书 v1；其发布请求返回 HTTP 403；`reviewer` 发布为 valid v2。
- 证书版本保留 `preparedBy=operator`、`verifiedBy=reviewer` 和请求 ID。
- 实体审计历史包含 `gb515-auth-create`、`gb515-auth-review`、`gb515-auth-approve`；审计汇总覆盖两个独立操作者。

### 内置 Browser

仅使用 Codex 内置 Browser，在 `http://127.0.0.1:18515` 实际验证：

- 登录页可选择 admin/reviewer/operator/viewer 并建立真实 JWT 会话。
- `/parts`、`/inspections`、`/certificates`、`/authorizations`、`/audit` 五个页面均加载成功。
- 在部件页通过确认对话框将 AP-001 从 received 推进到 inspection，页面刷新后状态正确。
- `PartStatusBadge` 在部件与放行页渲染；`CertificatePanel` 在检查与证书页展示版本、操作者和请求 ID。
- operator 在 review/approved 放行记录上只看到“等待复核员”，没有批准按钮；draft 仍可提交 review。
- viewer 不显示新增和状态推进按钮，只显示只读状态。
- 审计页展示 operator/reviewer 的创建、提交、批准、发布动作及对应请求 ID。
- 桌面全页截图和 390 x 844 移动端截图已检查；移动端表格采用稳定横向滚动，不挤压文字。
- 最终控制台 `error`/`warning` 日志为空。

### 清理

验证结束后执行：

```bash
docker compose down -v --remove-orphans
```

并确认没有名称包含 `aircraft-component-airworthiness-release` 的运行中容器。
