package thsxml

import (
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// 真实响应的形状参考：根元素名不固定，ret 与 item 平级，属性承载数据。
const sampleLoginOK = `<xml><ret code="0" msg="ok"/><item userid="123456" sessionid="ABCDEF" account="13800000000" rsa_version="default_5"/></xml>`

const samplePubKey = `<xml><ret code="0" msg=""/><item pubkey="-----BEGIN PUBLIC KEY-----&#10;MIIBIjANBg==&#10;-----END PUBLIC KEY-----&#10;" rsa_version="default_5"/></xml>`

const samplePassport = `<xml><ret code="0" msg=""/><item passport="userid=123456|signvalid=XYZ|token=abc"/></xml>`

const sampleFailure = `<xml><ret code="-1" msg="账号或密码错误"/></xml>`

func TestParseOK(t *testing.T) {
	resp, err := Parse([]byte(sampleLoginOK))
	if err != nil {
		t.Fatalf("Parse 报错: %v", err)
	}
	if resp.Ret.Code != "0" {
		t.Errorf("ret.code = %q", resp.Ret.Code)
	}
	if resp.Item.Attr("userid") != "123456" || resp.Item.Attr("sessionid") != "ABCDEF" {
		t.Errorf("item 属性不符: %+v", resp.Item.Attrs)
	}
}

func TestParsePubKeyPreservesNewlines(t *testing.T) {
	// PEM 公钥通过 &#10; 字符引用换行，解析器必须还原为真实换行。
	resp, err := Parse([]byte(samplePubKey))
	if err != nil {
		t.Fatalf("Parse 报错: %v", err)
	}
	pubkey := resp.Item.Attr("pubkey")
	if !strings.Contains(pubkey, "\n") {
		t.Errorf("公钥中的换行被吞掉: %q", pubkey)
	}
	if !strings.HasPrefix(pubkey, "-----BEGIN PUBLIC KEY-----") {
		t.Errorf("公钥头不符: %q", pubkey)
	}
}

func TestParsePassport(t *testing.T) {
	resp, err := Parse([]byte(samplePassport))
	if err != nil {
		t.Fatalf("Parse 报错: %v", err)
	}
	if resp.Item.Attr("passport") != "userid=123456|signvalid=XYZ|token=abc" {
		t.Errorf("passport 不符: %q", resp.Item.Attr("passport"))
	}
}

func TestParseBusinessFailure(t *testing.T) {
	_, err := Parse([]byte(sampleFailure))
	if err == nil {
		t.Fatal("业务失败应当报错")
	}
	apiErr, ok := err.(*APIFailure)
	if !ok {
		t.Fatalf("错误类型应为 *APIFailure，实际 %T", err)
	}
	if apiErr.Code != "-1" || apiErr.Msg != "账号或密码错误" {
		t.Errorf("错误内容不符: %+v", apiErr)
	}
}

func TestParseMissingRet(t *testing.T) {
	if _, err := Parse([]byte(`<xml><item userid="1"/></xml>`)); err == nil {
		t.Error("缺少 ret 节点应当报错")
	}
}

func TestParseGarbage(t *testing.T) {
	if _, err := Parse([]byte("not xml at all")); err == nil {
		t.Error("非 XML 输入应当报错")
	}
}

// TestParseGB2312Declared 模拟真实服务器：声明 GB2312、正文 GB 系编码。
func TestParseGB2312Declared(t *testing.T) {
	encoded, err := simplifiedchinese.GB18030.NewEncoder().Bytes(
		[]byte(`<xml><ret code="-1" msg="账号或密码错误"/></xml>`))
	if err != nil {
		t.Fatal(err)
	}
	declared := append([]byte(`<?xml version="1.0" encoding="GB2312"?>`), encoded...)
	_, parseErr := Parse(declared)
	apiErr, ok := parseErr.(*APIFailure)
	if !ok {
		t.Fatalf("应为 APIFailure: %T %v", parseErr, parseErr)
	}
	if apiErr.Msg != "账号或密码错误" {
		t.Errorf("GB2312 中文应正确解码: %q", apiErr.Msg)
	}

	okEncoded, _ := simplifiedchinese.GB18030.NewEncoder().Bytes(
		[]byte(`<xml><ret code="0" msg="成功"/><item userid="42"/></xml>`))
	okDeclared := append([]byte(`<?xml version="1.0" encoding="GBK"?>`), okEncoded...)
	resp, parseErr := Parse(okDeclared)
	if parseErr != nil {
		t.Fatalf("GBK 声明的成功响应应可解析: %v", parseErr)
	}
	if resp.Item.Attr("userid") != "42" {
		t.Errorf("item 属性不符: %+v", resp.Item.Attrs)
	}
}
