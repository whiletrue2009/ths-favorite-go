---
name: ths
description: 管理同花顺自选股：查看/添加/删除/重排自选股与自定义分组、生成分享链接。当用户提到"自选股""加自选""删自选""同花顺分组""调整自选顺序"等需求时使用。
---

# ths — 同花顺自选股管理技能

本地命令行工具 `ths`，管理用户自己的同花顺账号的自选股。数据只发往同花顺官方接口，
凭据存在用户本机。以下规则必须遵守。

## 安全规则（必须遵守）

1. **写操作两步走**：先加 `--dry-run` 预览给用户看，用户明确同意后再加 `--yes` 执行。
2. **不碰凭据**：登录让用户自己运行 `ths login`（隐藏输入）；需要代填时用
   `printf '账号\n密码\n' | ths login --stdin`。密码绝不进对话记录和文件。
3. **不越权**：只操作用户明确提到的分组；`readonly:true` 的动态分组只读；
   `reorder` 只允许调整顺序（成员必须与远端完全一致），不能夹带增删。
4. **删除分组**（`group del`）会连组内股票一起删，必须先提醒用户。
5. 出错先跑 `ths doctor`，把各项 `detail/remedy` 转述给用户。

## 命令速查

| 命令 | 用途 |
|---|---|
| `ths login` | 登录（首次必做；保存账号后 24h 自动续期） |
| `ths list` / `ths list -g 组名` | 全部分组 / 单组成员（含加入价与时间） |
| `ths self list` | 我的自选 |
| `ths stock add 组名 代码.市场 [--dry-run/--yes]` | 加股票（可多个） |
| `ths stock del 组名 代码.市场 [--dry-run/--yes]` | 删股票 |
| `ths reorder 组名 完整顺序 [--dry-run/--yes]` | 重排（先 `list -g` 拿现顺序） |
| `ths group add/del 名称 [--yes]` | 建组 / 删组 |
| `ths group share 组名 [秒]` | 分享链接（默认 7 天） |
| `ths doctor` | 自检（网络/登录态/凭据） |

股票代码格式：`600519.SH`（沪）`000001.SZ`（深）`430047.BJ`（北交所）`00700.HK`（港）。
市场后缀：SH/SZ/BJ/KC/HK/US。

## 输出契约

- stdout 恒为单行 JSON；成功含 `"ok":true`，失败 `{"ok":false,"code":"...","error":"中文说明"}`。
- 退出码：0 成功；1 参数错；2 网络（可重试）；3 业务拒绝（改请求）；4 未登录（需 `ths login`）。

## 示例

**例 1：用户说"看看我的自选股"**

```bash
ths self list
```
把返回的 `items`（symbol/price/added_at）整理成表格告诉用户。

**例 2：用户说"把 600519 和 000858 加到消费组"**

```bash
ths stock add 消费 600519.SH 000858.SZ --dry-run   # 先预览
# 把 dry-run 结果告诉用户，用户确认后：
ths stock add 消费 600519.SH 000858.SZ --yes
```

**例 3：用户说"我的自选里删掉平安银行"**

先确认代码（平安银行=000001.SZ），再：

```bash
ths stock del 我的自选 000001.SZ --dry-run
ths stock del 我的自选 000001.SZ --yes
```

**例 4：用户说"把科技组里的股票按今天涨幅排序"**

```bash
ths list -g 科技          # 1. 拿当前成员与顺序（行情数据本工具不提供，涨幅需用户告知或其他来源）
ths reorder 科技 A.SH B.SZ C.SZ --dry-run   # 2. 预览新顺序（成员必须完全一致只换顺序）
ths reorder 科技 A.SH B.SZ C.SZ --yes       # 3. 执行
```

**例 5：用户说"建个白酒组，把我上次说的那几只放进去，再发我个分享链接"**

```bash
ths group add 白酒 --yes
ths stock add 白酒 600519.SH 000568.SZ 000858.SZ --dry-run   # 确认后 --yes
ths group share 白酒
```

## 常见错误处理

| 现象 | 动作 |
|---|---|
| 退出码 4 / "请先运行 ths login" | 引导用户登录（保存过账号会自动重登，持续失败才手动 login） |
| "未找到分组" | `ths list` 拿准确分组名 |
| 代码格式错误 | 检查 `代码.市场` 后缀；北交所是 `.BJ` |
| 刚写入读出来还是旧的 | 服务端异步生效（约 1 分钟内），等 30 秒再读 |
| macOS 首次运行被拦 | `xattr -d com.apple.quarantine <ths路径>` |
