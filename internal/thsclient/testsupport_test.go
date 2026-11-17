package thsclient

import (
	"encoding/base64"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"

	"github.com/whiletrue2009/ths-favorite-go/internal/pbwire"
)

// buildBlockstockResponse 构造 multiStorage 下载响应：
// field1 varint count、field2 varint version、重复 field3 分组。
func buildBlockstockResponse(t *testing.T, version uint64, groupCount int) []byte {
	t.Helper()
	var msg []byte
	msg = pbwire.AppendVarintField(msg, 1, uint64(groupCount))
	msg = pbwire.AppendVarintField(msg, 2, version)
	for i := 0; i < groupCount; i++ {
		msg = pbwire.AppendBytesField(msg, 3, buildGroupMessage(t, i))
	}
	return msg
}

func buildGroupMessage(t *testing.T, index int) []byte {
	t.Helper()
	name := "消费"
	if index == 1 {
		name = "科技"
	}
	gbk, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(name))
	if err != nil {
		t.Fatal(err)
	}
	nameB64 := base64.StdEncoding.EncodeToString(gbk)
	var groupData []byte
	groupData = pbwire.AppendBytesField(groupData, 1, []byte(nameB64))
	groupData = pbwire.AppendBytesField(groupData, 3, []byte("600519|000858,17|33"))
	var group []byte
	group = pbwire.AppendVarintField(group, 1, uint64(index))
	group = pbwire.AppendBytesField(group, 3, groupData)
	return group
}

// buildUserCookie 构造 user Cookie：URL 转义的 base64，解码后冒号分段，
// 第 18 段（下标 17）是 sessionid。
func buildUserCookie(sessionID string) string {
	fields := make([]string, 18)
	for i := range fields {
		fields[i] = "f"
	}
	fields[17] = sessionID
	joined := strings.Join(fields, ":")
	return base64.RawStdEncoding.EncodeToString([]byte(joined))
}
