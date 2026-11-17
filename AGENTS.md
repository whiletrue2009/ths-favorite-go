# AGENTS.md — 给 AI Agent 的操作手册

你（AI agent）正在帮用户管理**同花顺自选股**。本文件是 `ths` 命令行工具的完整说明书。
请先读完「安全规则」和「输出契约」，再执行任何命令。

## 这个工具是什么

- `ths` 是一个本地命令行程序，通过同花顺官方接口管理用户的自选股分组。
- **所有网络请求只发往同花顺官方域名（`*.10jqka.com.cn`），不经过任何第三方服务器。**
- 账号密码保存在用户自己电脑的系统钥匙串（macOS Keychain / Windows 凭据管理器），登录态保存在用户配置目录（0600 权限）。
- 单用户单机使用：每个用户管理自己的同花顺账号，没有服务器、没有共享。

## 安全规则（必须遵守）

1. **写操作必须两步走**：先加 `--dry-run` 预览，把结果给用户看，用户明确同意后，再加 `--yes` 执行。不要跳过预览直接写。
2. **永远不要**把用户的账号密码写进命令行参数以外的对话记录、文件或日志。登录时优先让用户自己运行 `ths login`（交互式隐藏输入），或用 `printf '账号\n密码\n' | ths login --stdin`。
3. **永远不要**修改、删除或重排用户没有明确要求的分组。
4. `reorder`（重排）必须先 `ths list -g 分组名` 拿到当前完整成员列表，只调整顺序后原样提交——不能增删成员。
5. 删除分组（`group del`）会连组内股票一起删除，务必提醒用户。
6. 遇到任何错误，先运行 `ths doctor` 并把结果转述给用户，按 `remedy` 字段建议操作。

## 输出契约

- 成功：stdout 输出**单行 JSON**，含 `"ok": true`。
- 失败：stdout 输出单行 JSON `{"ok":false,"code":"...","error":"中文错误说明"}`，stderr 有人话提示。
- `--pretty` 标志可切换为人类可读表格（给人看时可用）。

退出码（用于程序化判断）：

| 退出码 | code 字段 | 含义 | agent 应该做什么 |
|---|---|---|---|
| 0 | — | 成功 | 解析 JSON，向用户汇报结果 |
| 1 | usage | 参数/用法错误 | 修正参数后重试 |
| 2 | network | 网络失败 | 等几秒重试一次；再失败跑 `ths doctor` |
| 3 | api | 同花顺业务拒绝（分组不存在、代码格式错等） | 读 error 字段修正请求 |
| 4 | auth | 未登录或登录态过期 | 引导用户运行 `ths login` |

## 命令参考

### ths login（首次使用必做）

```bash
# 方式一：交互式（推荐，用户自己运行，密码隐藏输入）
ths login

# 方式二：agent 通过 stdin 传入（第一行账号，第二行密码）
printf '13800000000\npassword123\n' | ths login --stdin
```

成功输出：`{"ok":true,"operation":"login","userid":"...","username":"...","group_count":5,...}`
登录后登录态 24 小时有效；账号保存后过期会**自动重登**，通常无需再次 login。

### ths list — 查看分组

```bash
ths list                    # 全部分组概览（含"我的自选"，动态分组自动查成员）
ths list -g 消费            # 单个分组的成员明细（含加入价格/时间）
ths list --pretty           # 人类表格
```

返回结构：`{"ok":true,"groups":[{"name":"消费","group_id":"0_12","readonly":false,"items":[{"symbol":"600519.SH","code":"600519","market":"SH","price":1688.5,"added_at":"20231101"}]}]}`

- `readonly: true` 的分组是**动态分组**（同花顺条件选股生成），不能增删改，只能看。
- `group_id` 为 `__selfstock__` 或名称为「我的自选」的是系统自选分组，可增删，不能删除分组本身。

### ths self list — 查看我的自选

`{"ok":true,"group":{"name":"我的自选","group_id":"__selfstock__","items":[...]}}`

### ths stock add / del — 添加/删除股票

```bash
# 两步走示例：添加
ths stock add 消费 600519.SH 000858.SZ --dry-run    # 预览
ths stock add 消费 600519.SH 000858.SZ --yes        # 执行

ths stock del 我的自选 000001.SZ --yes
```

股票代码格式：`代码.市场`。市场后缀：`SH` 上海、`SZ` 深圳、`BJ` 北交所、`KC` 科创板、`HK` 港股、`US` 美股。
分组用名称或 `ths list` 里看到的 `group_id` 都可以。

### ths reorder — 重排分组顺序

```bash
ths list -g 消费                       # 1. 先看当前成员和顺序
ths reorder 消费 000858.SZ 600519.SH --dry-run   # 2. 预览新顺序（成员必须完全一致）
ths reorder 消费 000858.SZ 600519.SH --yes       # 3. 执行
```

新列表必须是现有成员的**完整排列**：不能多、不能少、不能重复，只变顺序。

### ths group add / del / share — 分组管理

```bash
ths group add 长线跟踪 --yes
ths group del 消费 --dry-run     # 提醒：组内股票会一起删除
ths group del 消费 --yes
ths group share 消费             # 生成 7 天分享链接
ths group share 消费 86400       # 1 天有效
```

### ths doctor — 自检

`{"ok":true,"healthy":true,"version":"...","checks":[{"name":"network","ok":true,"detail":"...","remedy":""}]}`
检查项：network（连通性）、session（登录态）、credentials（保存的账号）、session_valid（服务端确认）。
每项失败的 `remedy` 都是可直接执行的修复建议。

### ths logout / ths version

登出会删除本地登录态和保存的账号密码。

## 常见问题处理

| 现象 | 处理 |
|---|---|
| `auth` 错误「请先运行 ths login」 | 引导用户登录（保存过账号的话多数命令会自动重登，持续失败才需要手动 login） |
| `api` 错误「未找到分组」 | 先 `ths list` 拿准确分组名 |
| 股票代码报错 | 检查 `代码.市场` 格式；北交所是 `.BJ` 不是 `.BSE` |
| macOS 首次运行被拦截 | 运行 `xattr -d com.apple.quarantine /path/to/ths`（或右键→打开） |
| Windows SmartScreen | 「更多信息」→「仍要运行」 |
| 网络持续失败 | `ths doctor`；检查代理设置 |
| 刚写入的分组读出来还是旧的 | 同花顺服务端写入是异步生效的（可达约 1 分钟）；工具已内置等待，若仍见旧数据，等 30 秒再读 |

## 环境信息

- 状态文件位置：`ths doctor` 的 session 检查会显示具体路径（默认 OS 用户配置目录下 `ths/session.json`）。
- 支持平台：Windows (x64/arm64)、macOS (Apple Silicon/Intel)、Linux (x64/arm64)。
- 本工具是开源软件（Apache-2.0），源码可审计。
