// Package pbwire 实现同花顺 multiStorage 接口使用的最小 protobuf wire 编解码。
// 只覆盖项目实际用到的两种 wire type：varint(0) 与 length-delimited(2)。
package pbwire

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	wireVarint = 0
	wireBytes  = 2
)

// EncodeVarint 编码无符号整数。
func EncodeVarint(value uint64) []byte {
	return binary.AppendUvarint(nil, value)
}

// AppendVarintField 追加一个 varint 字段。
func AppendVarintField(dst []byte, fieldNumber int, value uint64) []byte {
	dst = binary.AppendUvarint(dst, uint64(fieldNumber)<<3|wireVarint)
	return binary.AppendUvarint(dst, value)
}

// AppendBytesField 追加一个 length-delimited 字段。
func AppendBytesField(dst []byte, fieldNumber int, payload []byte) []byte {
	dst = binary.AppendUvarint(dst, uint64(fieldNumber)<<3|wireBytes)
	dst = binary.AppendUvarint(dst, uint64(len(payload)))
	return append(dst, payload...)
}

// Field 是解码出的单个字段。
type Field struct {
	Number int
	// Varint 仅在 WireType == 0 时有效。
	Varint uint64
	// Bytes 仅在 WireType == 2 时有效。
	Bytes []byte
	// WireType 为 0 或 2。
	WireType int
}

// Decode 解码一段消息体的全部顶层字段。
func Decode(data []byte) ([]Field, error) {
	var fields []Field
	offset := 0
	for offset < len(data) {
		tag, n := binary.Uvarint(data[offset:])
		if n <= 0 {
			return nil, errors.New("pbwire: 无效的 tag varint")
		}
		offset += n
		fieldNumber := int(tag >> 3)
		wireType := int(tag & 0x07)
		switch wireType {
		case wireVarint:
			value, n := binary.Uvarint(data[offset:])
			if n <= 0 {
				return nil, errors.New("pbwire: 无效的 varint 值")
			}
			offset += n
			fields = append(fields, Field{Number: fieldNumber, Varint: value, WireType: wireType})
		case wireBytes:
			length, n := binary.Uvarint(data[offset:])
			if n <= 0 {
				return nil, errors.New("pbwire: 无效的长度 varint")
			}
			offset += n
			if uint64(len(data)-offset) < length {
				return nil, fmt.Errorf("pbwire: 字段 %d 长度 %d 超出剩余数据 %d", fieldNumber, length, len(data)-offset)
			}
			fields = append(fields, Field{
				Number:   fieldNumber,
				Bytes:    data[offset : offset+int(length)],
				WireType: wireType,
			})
			offset += int(length)
		default:
			return nil, fmt.Errorf("pbwire: 不支持的 wire type %d（字段 %d）", wireType, fieldNumber)
		}
	}
	return fields, nil
}
