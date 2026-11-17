package thsclient

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"

	"golang.org/x/text/encoding/simplifiedchinese"

	"github.com/whiletrue2009/ths-favorite-go/internal/pbwire"
)

// multiStorage blockstock 协议（cs.10jqka.com.cn）：
// 承载全部分组（我的自选 + 自定义分组）的批量读与全量写。
// 请求为表单 + 可选二进制文件，响应为手写 protobuf wire 格式。
const (
	multiStorageAppName       = "blockstock"
	multiStorageDefaultClient = "hevo_pc"
	multiStorageStorePath     = "/"
)

// BlockstockGroup 是 multiStorage 下载返回的单个分组。
type BlockstockGroup struct {
	Name      string
	GroupType uint64
	Items     []StockEntry
}

// BlockstockDownload 是一次下载的完整结果。
type BlockstockDownload struct {
	Count   int
	Version uint64
	Groups  []BlockstockGroup
}

// BlockstockDownload 拉取远端全部分组与成员。
func (c *Client) BlockstockDownload(ctx context.Context, auth AuthParams) (*BlockstockDownload, error) {
	form := url.Values{
		"reqtype":    {"download"},
		"userid":     {auth.UserID},
		"storepath":  {multiStorageStorePath},
		"sessionid":  {auth.SessionID},
		"expires":    {auth.Expires},
		"appname":    {multiStorageAppName},
		"storetype":  {"2"},
		"clienttype": {multiStorageDefaultClient},
		"version":    {"0"},
	}
	body, err := c.PostForm(ctx, "blockstock download", MultiStorageURL, form)
	if err != nil {
		return nil, err
	}
	return parseBlockstockDownload(body)
}

// BlockstockUpload 全量上传单个分组（成员与顺序），返回新版本号。
// 线上事实：写入是异步应用的（读可能滞后 ~1 分钟）；被拒时响应退化为
// 3 字节（version=0），成功响应携带应用后的新版本号（>0）。
func (c *Client) BlockstockUpload(ctx context.Context, auth AuthParams, group BlockstockGroup, version string) (uint64, error) {
	body, err := c.postBlockstockUpload(ctx, auth, group, version)
	if err != nil {
		return 0, err
	}
	result, err := parseBlockstockDownload(body)
	if err != nil {
		return 0, err
	}
	if result.Version == 0 {
		return 0, &APIError{
			Action: "blockstock upload",
			Msg:    "服务端拒绝了写入（响应无新版本号），请重新拉取分组后再试",
		}
	}
	return result.Version, nil
}

// postBlockstockUpload 发送上传请求并返回原始响应（Debug 变体共用）。
func (c *Client) postBlockstockUpload(ctx context.Context, auth AuthParams, group BlockstockGroup, version string) ([]byte, error) {
	payload, err := encodeBlockstockPayload(group)
	if err != nil {
		return nil, err
	}
	form := url.Values{
		"appname":               {multiStorageAppName},
		"reqtype":               {"upload"},
		"version":               {version},
		"storepath":             {multiStorageStorePath},
		"clienttype":            {multiStorageDefaultClient},
		"compresstype":          {"none"},
		"compresstype_upload":   {"none"},
		"compresstype_download": {"none"},
		"userid":                {auth.UserID},
		"sessionid":             {auth.SessionID},
		"expires":               {auth.Expires},
	}
	return c.PostMultipart(ctx, "blockstock upload", MultiStorageURL, form, "uploadFile", "testFileList", payload, "application/octet-stream")
}

// DebugBlockstockUpload 与 BlockstockUpload 相同但返回原始响应字节（调试用）。
func (c *Client) DebugBlockstockUpload(ctx context.Context, auth AuthParams, group BlockstockGroup, version string) ([]byte, error) {
	return c.postBlockstockUpload(ctx, auth, group, version)
}

// encodeBlockstockPayload 构造上传 protobuf（字节结构与线上严格一致）：
//
//	field1 {
//	  field1 len {
//	    field1 varint group_type   ← 必须嵌套包裹，顶层 varint 会被服务端拒绝
//	  }
//	  field3 len {
//	    field1 len base64(gbk(name))
//	    field3 len "code|code,...,market|market"
//	  }
//	}
func encodeBlockstockPayload(group BlockstockGroup) ([]byte, error) {
	nameGBK, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(group.Name))
	if err != nil {
		return nil, fmt.Errorf("分组名转 GBK 编码失败: %w", err)
	}
	nameB64 := base64.StdEncoding.EncodeToString(nameGBK)

	var groupData []byte
	groupData = pbwire.AppendBytesField(groupData, 1, []byte(nameB64))
	groupData = pbwire.AppendBytesField(groupData, 3, []byte(FormatStockList(group.Items)))

	// group_type 包一层 field1 消息再放进分组消息。
	typeWrapped := pbwire.AppendVarintField(nil, 1, group.GroupType)
	var groupPayload []byte
	groupPayload = pbwire.AppendBytesField(groupPayload, 1, typeWrapped)
	groupPayload = pbwire.AppendBytesField(groupPayload, 3, groupData)

	return pbwire.AppendBytesField(nil, 1, groupPayload), nil
}

// parseBlockstockDownload 解析下载/上传响应：
//
//	field1 varint count
//	field2 varint version
//	field3 len group（重复）
//
// multiStorage 的存储里除股票分组外还混有客户端配置 blob（JSON/INI/纯文本），
// 这些 blob 的内容不是合法 protobuf——逐组容错：坏组跳过，好组保留。
func parseBlockstockDownload(data []byte) (*BlockstockDownload, error) {
	fields, err := pbwire.Decode(data)
	if err != nil {
		return nil, &NetError{Action: "blockstock", Detail: "响应 protobuf 解码失败: " + err.Error()}
	}
	result := &BlockstockDownload{}
	for _, field := range fields {
		switch field.Number {
		case 1:
			if field.WireType == 0 {
				result.Count = int(field.Varint)
			}
		case 2:
			if field.WireType == 0 {
				result.Version = field.Varint
			}
		case 3:
			if field.WireType == 2 {
				group, err := parseBlockstockGroup(field.Bytes)
				if err != nil {
					// 非股票分组的配置 blob，跳过。
					continue
				}
				result.Groups = append(result.Groups, group)
			}
		}
	}
	return result, nil
}

// parseBlockstockGroup 解析单个分组消息。组内数据（field 3）解码失败视为
// 配置 blob，返回错误由上层跳过。
func parseBlockstockGroup(data []byte) (BlockstockGroup, error) {
	fields, err := pbwire.Decode(data)
	if err != nil {
		return BlockstockGroup{}, &NetError{Action: "blockstock", Detail: "分组消息解码失败: " + err.Error()}
	}
	group := BlockstockGroup{}
	for _, field := range fields {
		switch field.Number {
		case 1:
			// group_type 可能是顶层 varint，也可能包一层消息。
			if field.WireType == 0 {
				group.GroupType = field.Varint
			} else if field.WireType == 2 {
				if inner, err := pbwire.Decode(field.Bytes); err == nil {
					for _, innerField := range inner {
						if innerField.Number == 1 && innerField.WireType == 0 {
							group.GroupType = innerField.Varint
						}
					}
				}
			}
		case 2:
			// 数值型分组 ID，当前协议未使用。
		case 3:
			if field.WireType != 2 {
				continue
			}
			inner, err := pbwire.Decode(field.Bytes)
			if err != nil {
				return BlockstockGroup{}, &NetError{Action: "blockstock", Detail: "分组数据不是有效 protobuf（可能是配置数据）: " + err.Error()}
			}
			for _, innerField := range inner {
				switch innerField.Number {
				case 1:
					name, err := decodeGroupName(string(innerField.Bytes))
					if err == nil {
						group.Name = name
					} else {
						group.Name = string(innerField.Bytes)
					}
				case 3:
					group.Items = ParseStockList(string(innerField.Bytes))
				}
			}
		}
	}
	return group, nil
}

// decodeGroupName 把 base64(GBK) 分组名还原为 UTF-8。
func decodeGroupName(b64 string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", err
	}
	decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(raw)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}
