# CLAUDE.md

面向 AI agent 的**使用手册在 [AGENTS.md](AGENTS.md)**，操作 `ths` CLI 前先读它。

本文件是**开发约定**。

## 项目

Go 语言干净重写的同花顺自选股管理 CLI（`ths`）。参考行为来自协议逆向（见 `docs/protocol.md`），
与 GPL 的 Python 参考实现（sunnysab/ths-favorite）保持**行为兼容但代码独立**，本仓库采用 Apache-2.0。

## 常用命令

```bash
go build ./...          # 编译
go test -race ./...     # 全部测试（必须带 -race）
go test -cover ./...    # 覆盖率
go vet ./...            # 静态检查
gofmt -w .              # 格式化（提交前必须干净）
go run . --help         # 本地试跑 CLI
```

发布：打 tag 后由 GitHub Actions（goreleaser）自动构建全平台二进制。版本号通过
`-ldflags "-X github.com/whiletrue2009/ths-favorite-go/internal/cliapp.Version=x.y.z"` 注入。

## 架构

```
main.go                  入口：错误 → 退出码
internal/cliapp/         cobra 命令装配（app/login/list/stock/group/doctor）
internal/engine/         业务编排：会话、分组解析、合并、dry-run 语义
                         （THSApi 接口在此定义，测试注入 fake）
internal/thsclient/      同花顺私有接口 HTTP 客户端（auth/groups/selfstock/
                         blockstock/dynamicplate/detail/probe）
internal/session/        登录态文件（0600，24h TTL）
internal/creds/          账号凭据（OS 钥匙串，文件回退）
internal/market/         市场代码 ↔ 缩写映射（BJ=151，兼容 71）
internal/pbwire/         最小 protobuf wire 编解码（multiStorage 用）
internal/thsxml/         verify2 XML 响应解析
internal/output/         输出契约：单行 JSON + 退出码语义
```

数据流：`cliapp → engine → thsclient → *.10jqka.com.cn`。

## 硬性约束

- **网络出口只能是 `*.10jqka.com.cn`**。新增任何第三方请求前必须先问用户。
- 错误分类决定退出码（见 `internal/output`）：网络 2 / 业务 3 / 登录态 4。新增错误路径时保持分类。
- 所有写操作走 engine 的 dry-run 语义；CLI 层强制 `--yes`。
- 凭据与 Cookie 永不进命令行参数、日志、错误消息。
- 市场代码、protobuf 字段号、接口路径都是线上抓包事实，改动前确认来源（docs/protocol.md）。
- 提交信息格式 `<type>: <描述>`（feat/fix/refactor/docs/test/chore/perf）。
- 不要提交 `session.json`、`credentials.json`（已在 .gitignore）。

## 测试

纯逻辑（market/pbwire/merge/doctor 判定）必须有 table-driven 单测；engine 用 fake THSApi
覆盖业务流；不写依赖真实同花顺账号的测试。
