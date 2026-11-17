// Package thsxml 解析同花顺 verify2 系列接口返回的 XML 报文。
// 同花顺响应普遍声明 encoding="GB2312"（正文为 GB 系编码），这里统一
// 用 GB18030 解码（GB2312/GBK 的超集）。
package thsxml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// Response 是 verify2 响应的结构。根元素名称不固定，只按子元素名匹配。
type Response struct {
	Ret  *Ret  `xml:"ret"`
	Item *Item `xml:"item"`
}

// Ret 是业务状态节点：code=0 表示成功。
type Ret struct {
	Code string `xml:"code,attr"`
	Msg  string `xml:"msg,attr"`
}

// Item 保留全部属性，不同步骤的属性集合不同
// （get_pubkey 返回 pubkey/rsa_version，unified_login 返回 userid/sessionid 等）。
type Item struct {
	Attrs map[string]string
}

// Attr 读取 item 属性。
func (i *Item) Attr(name string) string {
	if i == nil {
		return ""
	}
	return i.Attrs[name]
}

// rawItem 是解码中间结构，先取原始属性再转 map。
type rawItem struct {
	Attrs []xml.Attr `xml:",any,attr"`
}

type rawResponse struct {
	Ret  *Ret     `xml:"ret"`
	Item *rawItem `xml:"item"`
}

// Parse 解析并校验 verify2 响应，业务失败时返回错误。
func Parse(data []byte) (*Response, error) {
	var raw rawResponse
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.CharsetReader = charsetReader
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("响应不是有效 XML: %w（响应开头: %.120q）", err, data)
	}
	if raw.Ret == nil {
		return nil, fmt.Errorf("响应缺少 <ret> 节点: %.200s", strings.TrimSpace(string(data)))
	}
	if raw.Ret.Code != "0" {
		msg := raw.Ret.Msg
		if msg == "" {
			msg = "未知错误"
		}
		return nil, &APIFailure{Code: raw.Ret.Code, Msg: msg}
	}
	resp := &Response{Ret: raw.Ret}
	if raw.Item != nil {
		attrs := make(map[string]string, len(raw.Item.Attrs))
		for _, attr := range raw.Item.Attrs {
			attrs[attr.Name.Local] = attr.Value
		}
		resp.Item = &Item{Attrs: attrs}
	}
	return resp, nil
}

// APIFailure 表示 verify2 返回的非零业务码。
type APIFailure struct {
	Code string
	Msg  string
}

func (e *APIFailure) Error() string {
	return fmt.Sprintf("同花顺返回业务错误 code=%s: %s", e.Code, e.Msg)
}

// charsetReader 处理 GB 系字符集声明；未知字符集按原文透传（好过直接失败）。
func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	switch strings.ToLower(charset) {
	case "gb2312", "gbk", "gb18030":
		return transform.NewReader(input, simplifiedchinese.GB18030.NewDecoder()), nil
	default:
		return input, nil
	}
}
