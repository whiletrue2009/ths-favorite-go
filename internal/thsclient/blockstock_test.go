package thsclient

import (
	"encoding/hex"
	"testing"
)

func TestFormatAndParseStockListRoundtrip(t *testing.T) {
	entries := []StockEntry{
		{Code: "600519", MarketCode: "17"},
		{Code: "000001", MarketCode: "33"},
		{Code: "833171", MarketCode: "151"},
	}
	formatted := FormatStockList(entries)
	if formatted != "600519|000001|833171,17|33|151" {
		t.Fatalf("编码结果不符: %q", formatted)
	}
	parsed := ParseStockList(formatted)
	if len(parsed) != 3 {
		t.Fatalf("解析数量不符: %+v", parsed)
	}
	for i, entry := range parsed {
		if entry != entries[i] {
			t.Errorf("条目 %d 不符: %+v want %+v", i, entry, entries[i])
		}
	}
}

func TestParseStockListEmpty(t *testing.T) {
	if got := ParseStockList(""); got != nil {
		t.Errorf("空串应返回 nil，实际 %+v", got)
	}
	if got := ParseStockList("600519"); len(got) != 1 || got[0].Code != "600519" || got[0].MarketCode != "" {
		t.Errorf("缺市场码的解析不符: %+v", got)
	}
}

func TestBlockstockEncodeDecodeRoundtrip(t *testing.T) {
	group := BlockstockGroup{
		Name:      "消费",
		GroupType: 0,
		Items: []StockEntry{
			{Code: "600519", MarketCode: "17"},
			{Code: "000858", MarketCode: "33"},
		},
	}
	payload, err := encodeBlockstockPayload(group)
	if err != nil {
		t.Fatalf("编码失败: %v", err)
	}

	// 上传 payload 是 field1 包住的分组消息，先解包再复用 parseBlockstockGroup。
	outer, err := parseWrappedGroup(payload)
	if err != nil {
		t.Fatalf("解包失败: %v", err)
	}
	if outer.Name != group.Name {
		t.Errorf("组名往返不符: %q want %q（GBK 编码问题？）", outer.Name, group.Name)
	}
	if outer.GroupType != group.GroupType {
		t.Errorf("类型往返不符: %d", outer.GroupType)
	}
	if len(outer.Items) != 2 || outer.Items[0] != group.Items[0] || outer.Items[1] != group.Items[1] {
		t.Errorf("成员往返不符: %+v", outer.Items)
	}
}

func TestParseBlockstockDownloadMessage(t *testing.T) {
	// 构造与线上响应同构的消息：field1 varint count、field2 varint version、
	// 两个 field3 分组（其中一个用嵌套 varint 形态表达 group_type）。
	groupA := mustEncodeGroup(t, BlockstockGroup{Name: "我的自选", GroupType: 0, Items: []StockEntry{{Code: "600519", MarketCode: "17"}}})
	groupB := mustEncodeGroup(t, BlockstockGroup{Name: "科技", GroupType: 1, Items: nil})

	var msg []byte
	msg = appendVarintFieldForTest(msg, 1, 2)
	msg = appendVarintFieldForTest(msg, 2, 42)
	msg = appendBytesFieldForTest(msg, 3, groupA)
	msg = appendBytesFieldForTest(msg, 3, groupB)

	result, err := parseBlockstockDownload(msg)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if result.Count != 2 || result.Version != 42 {
		t.Errorf("count/version 不符: %d/%d", result.Count, result.Version)
	}
	if len(result.Groups) != 2 {
		t.Fatalf("分组数量不符: %d", len(result.Groups))
	}
	if result.Groups[0].Name != "我的自选" || len(result.Groups[0].Items) != 1 {
		t.Errorf("分组 0 不符: %+v", result.Groups[0])
	}
	if result.Groups[1].Name != "科技" {
		t.Errorf("分组 1 不符: %+v", result.Groups[1])
	}
}

func TestDecodeGroupNameInvalidB64(t *testing.T) {
	// 非法 base64 不应 panic，parseBlockstockGroup 回退为原样字符串。
	group := BlockstockGroup{Name: "!!!not-base64!!!", GroupType: 0}
	msg := mustEncodeGroup(t, group)
	parsed, err := parseBlockstockGroup(msg)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if parsed.Name == "" {
		t.Errorf("非法 base64 应回退为原样名称: %+v", parsed)
	}
}

func TestParseBlockstockDownloadGarbage(t *testing.T) {
	if _, err := parseBlockstockDownload([]byte{0xff, 0xff}); err == nil {
		t.Error("垃圾数据应当报错")
	}
}

// parseWrappedGroup 解开上传 payload 的 field1 包装。
func parseWrappedGroup(payload []byte) (BlockstockGroup, error) {
	fields, err := decodeForTest(payload)
	if err != nil {
		return BlockstockGroup{}, err
	}
	for _, field := range fields {
		if field.Number == 1 && field.WireType == 2 {
			return parseBlockstockGroup(field.Bytes)
		}
	}
	return BlockstockGroup{}, errNoFieldOne
}

var errNoFieldOne = &NetError{Action: "blockstock", Detail: "payload 缺少 field1"}

func TestSelfStockSymbol(t *testing.T) {
	entry := StockEntry{Code: "600519", MarketCode: "17"}
	if entry.Symbol() != "600519.SH" {
		t.Errorf("Symbol() = %q", entry.Symbol())
	}
	if (StockEntry{Code: "600519"}).Symbol() != "600519" {
		t.Errorf("无市场码 Symbol() 不符")
	}
}

func TestIsDynamic(t *testing.T) {
	if !(GroupMeta{ID: "1_abc"}).IsDynamic() {
		t.Error("1_ 前缀应为动态分组")
	}
	if (GroupMeta{ID: "0_3"}).IsDynamic() {
		t.Error("0_ 前缀应为普通分组")
	}
}

// TestParseBlockstockDownloadWithConfigBlobs 线上事实：multiStorage 响应中
// 混有非 protobuf 的配置 blob（JSON / 纯文本 userid / INI 配色表），
// 解析必须跳过它们并保留正常分组。
func TestParseBlockstockDownloadWithConfigBlobs(t *testing.T) {
	goodA := mustEncodeGroup(t, BlockstockGroup{Name: "消费", GroupType: 0,
		Items: []StockEntry{{Code: "600519", MarketCode: "17"}}})
	goodB := mustEncodeGroup(t, BlockstockGroup{Name: "科技", GroupType: 1})

	// 三个真实形态的 junk blob（伪造同构的字节）。
	jsonBlob := []byte(`{"idlist":{"234":[33619968]},"info":{"chunk":"\xd7\xf2\xc8"}`)
	iniBlob := []byte("[block_color_section]\r\n10=138,9309935,1755442000\r\n")
	textBlob := []byte("123456789")
	mk := func(payload []byte) []byte {
		// 组消息形态：field1{varint type} + field3{payload}（payload 本身不是 protobuf）
		var g []byte
		g = appendVarintFieldForTest(g, 1, 0)
		g = appendBytesFieldForTest(g, 3, payload)
		return g
	}

	var msg []byte
	msg = appendVarintFieldForTest(msg, 1, 4)
	msg = appendVarintFieldForTest(msg, 2, 3007)
	msg = appendBytesFieldForTest(msg, 3, goodA)
	msg = appendBytesFieldForTest(msg, 3, mk(jsonBlob))
	msg = appendBytesFieldForTest(msg, 3, goodB)
	msg = appendBytesFieldForTest(msg, 3, mk(iniBlob))
	msg = appendBytesFieldForTest(msg, 3, mk(textBlob))

	result, err := parseBlockstockDownload(msg)
	if err != nil {
		t.Fatalf("含配置 blob 的响应应解析成功: %v", err)
	}
	if result.Version != 3007 || result.Count != 4 {
		t.Errorf("顶层字段不符: %+v", result)
	}
	if len(result.Groups) != 2 {
		t.Fatalf("应保留 2 个正常分组（跳过 3 个 blob）: %+v", result.Groups)
	}
	if result.Groups[0].Name != "消费" || len(result.Groups[0].Items) != 1 {
		t.Errorf("分组 0 不符: %+v", result.Groups[0])
	}
	if result.Groups[1].Name != "科技" {
		t.Errorf("分组 1 不符: %+v", result.Groups[1])
	}
}

// TestUploadPayloadByteShape 与 Python 参考实现的线上验证字节严格一致。
// group_type 必须嵌套包裹（field1{field1 varint}），顶层 varint 会被服务端拒绝
// （2026-11 真机踩坑：响应退化为 3 字节、分组被追加空成员）。
func TestUploadPayloadByteShape(t *testing.T) {
	payload, err := encodeBlockstockPayload(BlockstockGroup{
		Name:      "自选测试",
		GroupType: 7,
		Items: []StockEntry{
			{Code: "600386", MarketCode: "17"},
			{Code: "600519", MarketCode: "17"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// 对照 Python field_bytes(1, field_varint(1,7) + field_bytes(3, field_bytes(1,b64)+field_bytes(3,stocks)))
	expected := "0a29" + // field1 len(41)
		"0a02" + "0807" + // field1 len(2){field1 varint 7}
		"1a23" + // field3 len(35)
		"0a0c" + hexOf("19TRobLiytQ=") + // field1 len(12) name b64
		"1a13" + hexOf("600386|600519,17|17") // field3 len(19) stocks
	if hex.EncodeToString(payload) != expected {
		t.Errorf("上传字节形状不符:\n got %s\nwant %s", hex.EncodeToString(payload), expected)
	}
}

func hexOf(s string) string {
	return hex.EncodeToString([]byte(s))
}
