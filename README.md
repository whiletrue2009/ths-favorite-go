# ths — 同花顺自选股管理工具

一个本地命令行工具：查看、添加、删除、重排你的同花顺自选股和自定义分组。
特别适合**让 AI（豆包、Codex、Claude、WorkBuddy 等）替你操作**——它输出结构化 JSON、
错误自带中文说明和修复建议。

## 特点

- ✅ **只连同花顺官方接口**（`*.10jqka.com.cn`），无任何第三方上传，可审计（开源）
- ✅ 账号密码存在**系统钥匙串**（macOS Keychain / Windows 凭据管理器），登录态 24 小时有效，过期自动重登
- ✅ 单文件程序，无需安装 Python/Docker/任何依赖
- ✅ AI agent 友好：单行 JSON 输出、语义化退出码、`--dry-run` 预览、`ths doctor` 自检
- ✅ 支持全部自选操作：分组增删、股票批量增删、组内排序、分组分享链接、动态分组（条件选股）查看

> ⚠️ 说明：本工具调用同花顺移动端私有接口（非官方开放 API），仅限个人自用；
> 密码经官方公钥加密后直连同花顺登录接口，不经任何中间服务器。

## 下载安装

从 [Releases](https://github.com/whiletrue2009/ths-favorite-go/releases) 下载对应平台的压缩包，解压得到 `ths`（Windows 为 `ths.exe`）。

| 平台 | 文件 |
|---|---|
| Windows x64 | `ths_*_windows_amd64.zip` |
| Windows ARM | `ths_*_windows_arm64.zip` |
| macOS Apple Silicon | `ths_*_darwin_arm64.zip` |
| macOS Intel | `ths_*_darwin_amd64.zip` |

**Windows**：解压后 SmartScreen 可能拦截 → 「更多信息」→「仍要运行」。

**macOS**：首次运行若被 Gatekeeper 拦截，在终端执行：

```bash
xattr -d com.apple.quarantine /path/to/ths
```

## 快速开始

```bash
# 1. 登录（账号密码只发往同花顺官方，本地保存在系统钥匙串）
./ths login

# 2. 看看你的分组
./ths list
./ths list -g 消费 --pretty

# 3. 添加 / 删除股票（--dry-run 先预览，--yes 才真执行）
./ths stock add 消费 600519.SH 000858.SZ --dry-run
./ths stock add 消费 600519.SH 000858.SZ --yes
./ths stock del 我的自选 000001.SZ --yes

# 4. 分组管理
./ths group add 长线跟踪 --yes
./ths group share 消费

# 5. 出问题先自检
./ths doctor
```

## 让 AI 帮你操作

把本仓库的 [AGENTS.md](AGENTS.md) 内容给任何 AI agent（或在项目目录里让它读），
然后直接说人话：

> 「帮我看看我的自选股分组」「把 600519 加到消费组」「把科技组按涨幅排一下」
> 「删掉消费组里的平安银行」「新建一个叫『白酒』的分组」

AI 会按安全规则操作：写操作先 dry-run 预览、经你确认后执行。

## 股票代码格式

`代码.市场`：`600519.SH`（沪）`000001.SZ`（深）`430047.BJ`（北交所）`00700.HK`（港）`AAPL.US`（美）

## 免责声明

本工具是个人学习研究用途的非官方客户端：使用你自己的同花顺账号通过官方登录接口访问自己的自选股数据，
不破解、不绕过任何验证机制，不抓取他人数据。使用非官方客户端可能不符合同花顺用户协议，由此产生的账号
风险（如限流、风控）由使用者自行承担；本工具不提供任何投资建议。请仅用于管理自己的账户数据。

## 许可

[Apache-2.0](LICENSE)。本项目是对同花顺客户端行为的独立观测实现，
与 GPL 许可的 Python 参考项目（[sunnysab/ths-favorite](https://github.com/sunnysab/ths-favorite)）行为兼容但代码独立。
