package pkg

import (
	"strings"
	"unicode/utf8"
)

// 判断字符串是否更像应用名
func isLikelyAppName(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	// 文件路径、扩展名一般不是应用名
	if strings.ContainsAny(s, "/\\.") {
		return false
	}
	// 太长的一般不是应用名
	if utf8.RuneCountInString(s) > 30 {
		return false
	}
	return true
}

// 清洗窗口标题，返回应用名
func CleanWindowTitle(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "Unknown"
	}

	// 常见分隔符
	separators := []string{"—", "-", ":"}
	for _, sep := range separators {
		if strings.Contains(raw, sep) {
			parts := strings.Split(raw, sep)
			left := strings.TrimSpace(parts[0])
			right := strings.TrimSpace(parts[len(parts)-1])

			if isLikelyAppName(right) {
				return right
			}
			if isLikelyAppName(left) {
				return left
			}
			// 都不像，优先返回右边（KDE常见风格）
			return right
		}
	}

	// 没有分隔符
	if isLikelyAppName(raw) {
		return raw
	}
	return "Unknown"
}
