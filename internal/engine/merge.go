package engine

import (
	"strings"

	"github.com/whiletrue2009/ths-favorite-go/internal/thsclient"
)

// mergeAdd 合并新增：按代码去重，新条目覆盖同代码旧条目，追加在尾部。
func mergeAdd(current, additions []thsclient.StockEntry) []thsclient.StockEntry {
	merged := make([]thsclient.StockEntry, 0, len(current)+len(additions))
	index := map[string]int{}
	for _, entry := range current {
		index[entry.Code] = len(merged)
		merged = append(merged, entry)
	}
	for _, entry := range additions {
		if existingIdx, found := index[entry.Code]; found {
			merged[existingIdx] = entry
			continue
		}
		index[entry.Code] = len(merged)
		merged = append(merged, entry)
	}
	return merged
}

// mergeDelete 合并删除：按代码剔除。
func mergeDelete(current, deletions []thsclient.StockEntry) []thsclient.StockEntry {
	remove := make(map[string]struct{}, len(deletions))
	for _, entry := range deletions {
		remove[entry.Code] = struct{}{}
	}
	merged := make([]thsclient.StockEntry, 0, len(current))
	for _, entry := range current {
		if _, deleted := remove[entry.Code]; deleted {
			continue
		}
		merged = append(merged, entry)
	}
	return merged
}

// sameMultiset 判断两组条目作为多重集合是否相等（忽略顺序）。
func sameMultiset(a, b []thsclient.StockEntry) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[thsclient.StockEntry]int, len(a))
	for _, entry := range a {
		counts[entry]++
	}
	for _, entry := range b {
		counts[entry]--
		if counts[entry] < 0 {
			return false
		}
	}
	return true
}

// multisetDiff 返回 (请求新增数, 请求缺失数)：
// 请求里有而远端没有的 = 新增；远端有而请求没有的 = 缺失。
func multisetDiff(remote, request []thsclient.StockEntry) (missing, extra int) {
	remoteCounts := make(map[thsclient.StockEntry]int, len(remote))
	for _, entry := range remote {
		remoteCounts[entry]++
	}
	requestCounts := make(map[thsclient.StockEntry]int, len(request))
	for _, entry := range request {
		requestCounts[entry]++
	}
	for entry, count := range requestCounts {
		if remoteCounts[entry] < count {
			missing += count - remoteCounts[entry]
		}
	}
	for entry, count := range remoteCounts {
		if requestCounts[entry] < count {
			extra += count - requestCounts[entry]
		}
	}
	return missing, extra
}

// IsVersionConflict 识别同花顺的版本冲突报错（英文与中文特征词）。
func IsVersionConflict(apiErr *thsclient.APIError) bool {
	code := strings.ToLower(apiErr.Code)
	if code == "409" || code == "version_conflict" {
		return true
	}
	msg := apiErr.Msg
	lowered := strings.ToLower(msg)
	enTokens := []string{"outdated", "mismatch", "refresh", "expired", "stale", "conflict"}
	if strings.Contains(lowered, "version") {
		for _, token := range enTokens {
			if strings.Contains(lowered, token) {
				return true
			}
		}
	}
	cnTokens := []string{"过期", "失效", "不一致", "不匹配", "刷新", "版本冲突", "版本过期"}
	if strings.Contains(msg, "版本") {
		for _, token := range cnTokens {
			if strings.Contains(msg, token) {
				return true
			}
		}
	}
	return false
}
