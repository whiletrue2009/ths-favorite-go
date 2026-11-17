package thsclient

import (
	"testing"

	"github.com/whiletrue2009/ths-favorite-go/internal/pbwire"
)

func mustEncodeGroup(t *testing.T, group BlockstockGroup) []byte {
	t.Helper()
	payload, err := encodeBlockstockPayload(group)
	if err != nil {
		t.Fatalf("编码分组失败: %v", err)
	}
	// 解出 field1 的内层分组消息。
	fields, err := pbwire.Decode(payload)
	if err != nil || len(fields) != 1 {
		t.Fatalf("解包失败: %v %+v", err, fields)
	}
	return fields[0].Bytes
}

func appendVarintFieldForTest(dst []byte, field int, value uint64) []byte {
	return pbwire.AppendVarintField(dst, field, value)
}

func appendBytesFieldForTest(dst []byte, field int, payload []byte) []byte {
	return pbwire.AppendBytesField(dst, field, payload)
}

func decodeForTest(data []byte) ([]pbwire.Field, error) {
	return pbwire.Decode(data)
}
