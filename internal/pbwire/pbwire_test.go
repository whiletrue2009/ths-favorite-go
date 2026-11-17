package pbwire

import (
	"bytes"
	"testing"
)

func TestAppendAndDecodeRoundtrip(t *testing.T) {
	var buf []byte
	buf = AppendVarintField(buf, 1, 3)
	buf = AppendBytesField(buf, 3, []byte("hello"))
	buf = AppendVarintField(buf, 2, 123456789)
	buf = AppendBytesField(buf, 3, []byte("world"))

	fields, err := Decode(buf)
	if err != nil {
		t.Fatalf("Decode 报错: %v", err)
	}
	if len(fields) != 4 {
		t.Fatalf("期望 4 个字段，实际 %d", len(fields))
	}

	if fields[0].Number != 1 || fields[0].WireType != 0 || fields[0].Varint != 3 {
		t.Errorf("字段 0 不符: %+v", fields[0])
	}
	if fields[1].Number != 3 || string(fields[1].Bytes) != "hello" {
		t.Errorf("字段 1 不符: %+v", fields[1])
	}
	if fields[2].Varint != 123456789 {
		t.Errorf("字段 2 varint 不符: %d", fields[2].Varint)
	}
	if string(fields[3].Bytes) != "world" {
		t.Errorf("字段 3 不符: %+v", fields[3])
	}
}

func TestNestedRoundtrip(t *testing.T) {
	// 模拟 blockstock 分组消息：field1 varint + field3 嵌套消息
	var inner []byte
	inner = AppendBytesField(inner, 1, []byte("aGVsbG8="))
	inner = AppendBytesField(inner, 3, []byte("600519|000001,17|33"))

	var group []byte
	group = AppendVarintField(group, 1, 0)
	group = AppendBytesField(group, 3, inner)

	payload := AppendBytesField(nil, 1, group)

	top, err := Decode(payload)
	if err != nil {
		t.Fatalf("Decode 报错: %v", err)
	}
	if len(top) != 1 || top[0].Number != 1 {
		t.Fatalf("顶层结构不符: %+v", top)
	}
	groupFields, err := Decode(top[0].Bytes)
	if err != nil {
		t.Fatalf("嵌套 Decode 报错: %v", err)
	}
	if len(groupFields) != 2 || groupFields[0].Varint != 0 {
		t.Fatalf("分组结构不符: %+v", groupFields)
	}
	dataFields, err := Decode(groupFields[1].Bytes)
	if err != nil {
		t.Fatalf("内层 Decode 报错: %v", err)
	}
	if string(dataFields[0].Bytes) != "aGVsbG8=" || string(dataFields[1].Bytes) != "600519|000001,17|33" {
		t.Fatalf("内层数据不符: %+v", dataFields)
	}

	// 与 Python 参考实现字节级对齐验证：
	// field_bytes(1, field_varint(1, 0) + field_bytes(3, field_bytes(1, b64) + field_bytes(3, stocks)))
	// 外层 field1 长度 35（2+2+10+21），内层 field3 长度 31（10+21），stocks 长度 19。
	expected := []byte{0x0a, 0x23, 0x08, 0x00, 0x1a, 0x1f, 0x0a, 0x08}
	expected = append(expected, []byte("aGVsbG8=")...)
	expected = append(expected, 0x1a, 0x13)
	expected = append(expected, []byte("600519|000001,17|33")...)
	if !bytes.Equal(payload, expected) {
		t.Errorf("编码字节与预期不一致:\n got %x\nwant %x", payload, expected)
	}
}

func TestDecodeTruncated(t *testing.T) {
	buf := AppendBytesField(nil, 3, []byte("0123456789"))
	if _, err := Decode(buf[:len(buf)-4]); err == nil {
		t.Error("截断的数据应当报错")
	}
	if _, err := Decode([]byte{0xff}); err == nil {
		t.Error("孤立的 tag 字节应当报错")
	}
}
