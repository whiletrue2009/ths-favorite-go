# 同花顺自选股私有接口协议规格

本文档记录 `ths` 所依赖的同花顺（10jqka.com.cn）移动端/PC 端私有接口的**线上行为事实**。
这些行为通过对官方客户端网络流量的独立观测归纳，是实现独立、行为兼容客户端的依据。
字段均为线上事实，变更需重新抓包验证。

## 总览

| 域名 | 用途 | 协议形态 |
|---|---|---|
| `auth.10jqka.com.cn` | 登录三步（RSA 公钥 / 统一登录 / mainverify） | XML |
| `upass.10jqka.com.cn` | userid+sessionid+signvalid 换 Cookie | 302 + Set-Cookie |
| `ugc.10jqka.com.cn` | 分组元数据 CRUD、分享、自选 v1 全量替换、自选明细 | JSON / XML |
| `t.10jqka.com.cn` | 自选 v2（查询 + 单只增删） | JSON |
| `cs.10jqka.com.cn` | multiStorage：全部分组批量读 / 全量写（blockstock） | 表单 + protobuf |
| `apigate.10jqka.com.cn` | 动态板块（条件选股分组）成员查询 | JSON |

User-Agent（数据接口，模拟安卓客户端）：

```
Hexin_Gphone/11.28.03 (Royal Flush) hxtheme/0 innerversion/G037.09.028.1.32 followPhoneSystemTheme/0 userid/000000000 getHXAPPAccessibilityMode/0 hxNewFont/1 isVip/0 getHXAPPFontSetting/normal getHXAPPAdaptOldSetting/0 okhttp/3.14.9
```

User-Agent（登录链路，模拟 iOS 客户端）：`同花顺/7.0.10 CFNetwork/1333.0.4 Darwin/21.5.0`

## 市场代码

股票以 `代码|市场数字码` 表示。常用：SH=17、SZ=33、KC=18、HK=55、US=61、指数=48。
北交所：**上行写操作使用 151**；下行响应中 71 与 151 都表示北交所（71 为历史码）。
ETF：沪 20、深 36。完整映射见 `internal/market/market.go`。

股票列表通用串格式：`code|code|code,market|market|market`（最后一个逗号分隔代码段与市场段，
段内用 `|`）。分组 content、自选 v1、multiStorage group data 均用此格式。

## 登录流程（四步）

### 1. 获取 RSA 公钥

`GET https://auth.10jqka.com.cn/verify2?reqtype=do_rsa&type=get_pubkey`

响应 XML：`<xml><ret code="0" msg=""/><item pubkey="-----BEGIN PUBLIC KEY-----&#10;...&#10;-----END PUBLIC KEY-----&#10;" rsa_version="default_5"/></xml>`
（PEM 公钥放在 XML 属性里，换行以 `&#10;` 字符引用编码；`ret code != 0` 为业务失败）

### 2. 统一登录

账号与密码分别用上述 RSA 公钥做 **PKCS1v15** 加密后 base64，作为查询参数：

`GET /verify2?account=<b64>&msg=1&passwd=<b64>&reqtype=unified_login&rsa_version=<v>&ta_appid=2022021114090152`

响应 item 属性：`userid`、`sessionid`、`account`、（可能更新的）`rsa_version`。

### 3. mainverify

`GET /verify2?reqtype=mainverify&userid=..&sessionid=..&qsid=8003&product=S01&version=11.4.1.3&imei=ZjI6MDY6NGE6NzI6MjQ6NTA=&sdsn=&rsa_version=..&nohqlist=0&securities=%E5%90%8C%E8%8A%B1%E9%A1%BA%E8%BF%9C%E8%88%AA%E7%89%88`

（`securities` 是 URL 编码的「同花顺远航版」；线上实际会二次编码，服务端接受）

响应 item 属性 `passport`，格式 `k=v|k=v|...`，取 `signvalid`。

### 4. 换取 Cookie

`GET https://upass.10jqka.com.cn/docookie2.php?userid=..&sessionid=..&signvalid=..`

响应 `Set-Cookie` 即正式会话 Cookie（关键项：`userid`、`user`、`sessionid` 等）。
Cookie 有效期约 24 小时。

### multiStorage 鉴权参数（从 Cookie 推导）

- `userid`：Cookie `userid`
- `sessionid`：Cookie `user` 的 URL 解码 → base64 解码 → 按 `:` 分段取第 18 段（下标 17）。
  **multiStorage 写操作只认这个 sessionid**（登录流程的 sessionid 仅下载可用）
- `expires`：当前时间 + 24h，格式 `YYYY-MM-DD HH:MM:SS`

## 分组元数据（JSON）

`GET https://ugc.10jqka.com.cn/optdata/selfgroup/open/api/group/v1/query?from=sjcg_gphone&types=0,1`

响应：`{"status_code":0,"data":{"version":"105","group_list":[{"id":"0_12","name":"消费","type":"0","num":"5","content":"600519|000858,17|33"}]}}`

- `id` 前缀：`0_` 普通自定义分组；`1_` **动态板块分组（只读，成员由条件实时生成）**
- `content` 即成员列表串；`version` 用于写操作乐观并发控制（**数字类型**，非字符串）
- `status_code != 0` 为业务失败，`status_msg` 为中文错误

写端点（POST `application/x-www-form-urlencoded`，均带 `version` 与 `from=sjcg_gphone`）：

- 建组：`/optdata/selfgroup/open/api/group/v1/add`，字段 `name`、`type=0`
- 删组：`/optdata/selfgroup/open/api/group/v1/delete`，字段 `ids=<group_id>`
- 分享：`POST /optdata/sharing_service/open/api/sharing/v1/create`（JSON），
  `{"biz":"selfstock","valid_time":<秒>,"biz_key":"<userid>_<id 中 _ 之后部分>","name":<组名>,"url_style":0}`，
  响应 `data.share_url`

版本冲突：服务端返回 code 409 / `version_conflict` 或消息含「版本过期/不一致」等特征词；
重取 version 重试一次即可。

## 「我的自选」

### v2（查询 + 单只增删，t.10jqka.com.cn）

- 查询：`GET /newcircle/group/getSelfStockWithMarket/` →
  `{"errorCode":0,"result":[{"code":"600519","marketid":"17"}]}`（**保持服务端顺序**）
- 单只增删：`GET /newcircle/group/modifySelfStock/?op=add|del&stockcode=<code>_<marketcode>`

### v1（全量替换，ugc.10jqka.com.cn）

- 查询：`GET /optdata/selfstock/open/api/v1/query?support_all=0&from=thspc_hevo`（请求头带 `userid`）→
  `data.selfstock`（成员串）+ `data.version`
- 替换：`POST /optdata/selfstock/open/api/v1/modify`，
  字段 `selfstock=<成员串>&from=thspc_hevo&version=<v>&num=<n>`（body 预 urlencode）

## multiStorage blockstock（全部分组批量读/写）

`POST https://cs.10jqka.com.cn/multiStorage`（multipart）

### 下载（表单字段）

`reqtype=download&userid&storepath=/&sessionid&expires&appname=blockstock&storetype=2&clienttype=hevo_pc&version=0`

响应为手写 protobuf wire（varint=0 / len-delim=2 两种 wire type）：

```
field1 varint   分组数
field2 varint   版本号
field3 bytes    分组消息（重复）
  ├ field1 varint   group_type（也可能包一层 field1 bytes{field1 varint}）
  └ field3 bytes    分组数据
      ├ field1 bytes  base64(GBK(分组名))
      └ field3 bytes  成员列表串
```

### 上传

`reqtype=upload&version=<下载时的版本>&storepath=/&clienttype=hevo_pc&compresstype=none&compresstype_upload=none&compresstype_download=none&userid&sessionid&expires`
+ 文件字段 `uploadFile`（filename=`testFileList`，`application/octet-stream`），内容为：

```
field1 bytes
  ├ field1 bytes      ← group_type 必须再包一层消息！
  │   └ field1 varint group_type
  └ field3 bytes
      ├ field1 bytes  base64(GBK(分组名))
      └ field3 bytes  成员列表串
```

响应同下载格式（含新版本号）。**分组名必须 GBK 编码后 base64**。

2026-11 真机实测补充（均为踩坑验证过的事实）：

1. **group_type 必须嵌套包裹**（`field_bytes(1, field_varint(1, type))`）。
   写成顶层 varint 时服务端不报错但写入错乱：响应退化为 3 字节（`08 03`），
   分组被追加空成员。
2. **鉴权必须用 user Cookie 里的 sessionid**（base64 解码后第 18 段），
   不是登录流程返回的 sessionid——下载对两者都宽容，上传只认前者。
3. **写入是异步应用的**：上传被接受后，读取路径可能滞后数十秒才可见；
   连续写操作必须轮询等待版本推进（本实现内置，约 20 秒上限）。
4. **被拒的上传**返回 3 字节响应（version=0），成功响应 version>0。
5. 分组消息里还有 field2 varint（数值分组标识，对应 JSON 接口 id `0_<数字>`
   的后缀，每账号各不相同）；分组数据里 field1（组名）与 field3（成员）
   之间还有 field2（空串），均可忽略但上传时保持 field1/field3 原样即可。
6. 下载响应中混有非 protobuf 的配置 blob（JSON / 纯文本 userid / INI 配色表），
   解析需逐组容错跳过。

## 动态板块成员

`GET https://apigate.10jqka.com.cn/d/platform/dynamicplate/stocks/self/v2/select?query=<URL编码组名>&num=1000`

→ `{"data":{"codes":[{"code":"600519","market":"17"}]}}`。`data` 缺失视为空板块；
瞬时无效响应应有界重试，最终失败不得伪装为空列表。

## 自选明细（价格/加入时间）

`GET https://ugc.10jqka.com.cn/selfstock_detail?reqtype=download&app_flag=0E&userid=<id>`
（请求头另带 `userid`）

响应 XML `<item version="107" selfstock_detail="<base64(JSON数组)>"/>`，
数组元素：`{"C":"600519","M":"17","P":1688.5,"T":"20231101"}`。
此接口尽力而为：失败时主列表仍可用，只是缺价格/时间。

## 安全注意

- 全部端点均为 HTTPS + 官方域名。任何实现不得引入第三方上报。
- 密码只在第 2 步以官方公钥 RSA 加密形式上行，本地明文只允许存在于钥匙串/0600 文件。
- 本工具模拟官方客户端行为；同花顺侧风控/限流策略不可控，异常时应该停止重试并提示用户。
