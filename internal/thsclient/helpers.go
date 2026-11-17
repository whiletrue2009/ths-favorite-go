package thsclient

import "strings"

// cutStockList 按「最后一个逗号」切分 "codes,types"。
// 股票代码与市场码本身都不含逗号，首个与末个逗号等价，统一取末个与 v1 协议对齐。
func cutStockList(content string) (codesSegment, typesSegment string) {
	if idx := strings.LastIndex(content, ","); idx >= 0 {
		return content[:idx], content[idx+1:]
	}
	return content, ""
}

// splitNonEmpty 按 sep 切分并丢弃空段。
func splitNonEmpty(s string, sep byte) []string {
	parts := strings.Split(s, string(sep))
	out := parts[:0]
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// orDefault 空字符串回退。
func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
