# ths — 给你的 AI 装上「同花顺自选股管理」技能

装好之后，你只需要**对 AI 说人话**：

> "帮我看看自选股" · "把 600519 加到消费组" · "科技组按我说的顺序排一下" · "建个白酒组再发我个分享链接"

AI（Claude、豆包、Codex 等）会替你操作同花顺自选股：写操作先预览、你点头才执行。

## 三步上手

**① 安装**（下载一个文件，无需 Python/Docker）

Windows（PowerShell）：

```powershell
irm https://raw.githubusercontent.com/whiletrue2009/ths-favorite-go/main/install.ps1 | iex
```

macOS / Linux（终端）：

```bash
curl -fsSL https://raw.githubusercontent.com/whiletrue2009/ths-favorite-go/main/install.sh | bash
```

国内直连 GitHub 慢的话用镜像：

```bash
curl -fsSL https://cdn.jsdelivr.net/gh/whiletrue2009/ths-favorite-go@main/install.sh | bash
```
（安装脚本内置了下载失败自动切镜像的回退；或者直接找朋友要 zip 安装包解压也行）

或者手动：到 [Releases](https://github.com/whiletrue2009/ths-favorite-go/releases) 下载对应平台的 zip 解压。
Windows 遇到 SmartScreen → 「更多信息」→「仍要运行」。

**② 登录**（账号密码只发往同花顺官方，保存在自己电脑的系统钥匙串）

```
ths login
```

**③ 教会你的 AI**

- **Claude Code**：`ths skill install`，重启后自动获得本技能
- **豆包 / 其他 AI**：把 `ths skill show` 输出的内容粘贴给它
- **Codex 等**：把输出放进你项目的 `AGENTS.md`

## 试试这些话

| 你说 | AI 会做 |
|---|---|
| "看看我的自选股" | `ths self list` 并整理成表格 |
| "把 600519、000858 加到消费组" | 先 `--dry-run` 给你看，你说"好"再 `--yes` 执行 |
| "自选里删掉平安银行" | 确认代码 000001.SZ 后两步删除 |
| "科技组按涨幅从高到低排一下"（涨幅你提供或另接数据源） | `ths list -g 科技` 拿现顺序 → `ths reorder` 调整 |
| "建个白酒组，放茅台五粮液，发我分享链接" | 建组 → 加股 → `ths group share 白酒` |

完整命令与 AI 安全规则见 [AGENTS.md](AGENTS.md)。

## 特点

- ✅ **只连同花顺官方接口**（`*.10jqka.com.cn`），无第三方上传，[开源](LICENSE)可审计
- ✅ 账号密码存**系统钥匙串**（macOS Keychain / Windows 凭据管理器），登录态 24 小时自动续期
- ✅ AI 友好契约：单行 JSON、语义退出码、写操作强制"预览→确认"两步、`ths doctor` 自检
- ✅ 全功能：分组增删、股票批量增删、组内重排、分享链接、动态分组（条件选股）查看
- ✅ 单文件静态二进制：Windows / macOS / Linux 全平台

> ⚠️ 服务端写入为异步生效（约 1 分钟内），工具已内置等待；若连续操作后读到旧数据，稍等片刻再读。

## 股票代码格式

`代码.市场`：`600519.SH`（沪）`000001.SZ`（深）`430047.BJ`（北交所）`00700.HK`（港）`AAPL.US`（美）

## 免责声明

本工具是个人学习研究用途的非官方客户端：使用你自己的同花顺账号通过官方登录接口访问自己的自选股数据，
不破解、不绕过任何验证机制，不抓取他人数据。使用非官方客户端可能不符合同花顺用户协议，由此产生的账号
风险（如限流、风控）由使用者自行承担；本工具不提供任何投资建议。请仅用于管理自己的账户数据。

## 赞助

如果你觉得好用那是 AI 的功劳，欢迎给作者支付宝赞助好买 token

<p align="center">
  <img src="docs/donate/alipay.jpg" width="240" alt="支付宝收款码" />
</p>



## 许可

[Apache-2.0](LICENSE)。本项目为独立实现。