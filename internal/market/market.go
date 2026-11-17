// Package market 实现同花顺市场代码与人类可读缩写之间的双向映射。
// 这些数字代码是同花顺私有接口的线上事实，变更需以抓包为准。
package market

import (
	"fmt"
	"strings"
)

// Abbr 返回市场缩写（如 "SH"），未知代码原样返回。
func Abbr(code string) string {
	if code == "" {
		return code
	}
	if name, ok := codeToAbbr[code]; ok {
		return name
	}
	return code
}

// Code 返回市场缩写对应的上行数字代码，未知缩写报错。
// 上行（写操作）只接受明确的常见缩写，避免歧义。
func Code(abbr string) (string, error) {
	if abbr == "" {
		return "", fmt.Errorf("市场后缀为空，请使用 code.market 格式，如 600519.SH")
	}
	upper := strings.ToUpper(abbr)
	if code, ok := abbrToCode[upper]; ok {
		return code, nil
	}
	return "", fmt.Errorf("未知的市场后缀 %q，常用：SH 上海、SZ 深圳、BJ 北交所、KC 科创板、HK 港股、US 美股", abbr)
}

// abbrToCode 是写操作使用的编码方向。
// 注意 BJ（北交所）上行使用 151；71 为旧接口历史码，仅识别不发送。
var abbrToCode = map[string]string{
	"SH":    "17",
	"SHETF": "20",
	"ST":    "22",
	"SZ":    "33",
	"SZETF": "36",
	"ZS":    "48",
	"CYB":   "38",
	"KC":    "18",
	"BJ":    "151",
	"HK":    "55",
	"US":    "61",
	"FT":    "50",
	"QH":    "51",
	"QZ":    "53",
	"OP":    "79",
	"JJ":    "39",
	"ZQ":    "45",
	"XSB":   "67",
}

// codeToAbbr 是读操作使用的解码方向；71 与 151 都归一化为 BJ。
var codeToAbbr = func() map[string]string {
	m := make(map[string]string, len(abbrToCode)+1)
	for abbr, code := range abbrToCode {
		m[code] = abbr
	}
	m["71"] = "BJ" // 北交所旧码，兼容历史接口返回
	return m
}()
