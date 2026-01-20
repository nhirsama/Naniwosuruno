package pkg

import (
	"strings"
	"unicode/utf8"
)

// 常见应用名白名单（全小写用于匹配）
// 这些应用名如果在标题片段中被发现，将直接被认为是正确的应用名
var knownApps = map[string]bool{
	"google chrome":      true,
	"chrome":             true,
	"chromium":           true,
	"firefox":            true,
	"mozilla firefox":    true,
	"brave":              true,
	"edge":               true,
	"microsoft edge":     true,
	"opera":              true,
	"vivaldi":            true,
	"visual studio code": true,
	"vs code":            true,
	"code":               true,
	"vscodium":           true,
	"sublime text":       true,
	"atom":               true,
	"android studio":     true,
	"intellij idea":      true,
	"goland":             true,
	"pycharm":            true,
	"webstorm":           true,
	"phpstorm":           true,
	"rider":              true,
	"clion":              true,
	"konsole":            true,
	"terminal":           true,
	"gnome-terminal":     true,
	"alacritty":          true,
	"kitty":              true,
	"wezterm":            true,
	"dolphin":            true,
	"thunar":             true,
	"nautilus":           true,
	"spotify":            true,
	"vlc media player":   true,
	"vlc":                true,
	"mpv":                true,
	"discord":            true,
	"slack":              true,
	"telegram":           true,
	"obs":                true,
	"obs studio":         true,
	"kdenlive":           true,
	"gimp":               true,
	"blender":            true,
	"libreoffice":        true,
	"kate":               true,
	"kwrite":             true,
	"steam":              true,
}

// 检查是否为已知应用名
func isKnownApp(s string) bool {
	return knownApps[strings.ToLower(s)]
}

// 判断字符串是否更像应用名
func isLikelyAppName(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}

	// 如果是白名单中的应用，直接返回 true
	if isKnownApp(s) {
		return true
	}

	// 路径一般不是应用名
	if strings.ContainsAny(s, "/\\") {
		return false
	}

	// 过于短的通常不是应用名（除非在白名单里）
	if utf8.RuneCountInString(s) < 2 {
		return false
	}

	// 太长的一般不是应用名，放宽限制到 60
	if utf8.RuneCountInString(s) > 60 {
		return false
	}

	// 如果包含扩展名分隔符 . ，且不在白名单中，通常认为是文件名
	// 但有些应用名可能包含点，这里做一个简单的启发式：
	// 如果点在末尾附近（像是扩展名），则认为不是应用名
	lastDotIndex := strings.LastIndex(s, ".")
	if lastDotIndex != -1 && lastDotIndex > len(s)-5 {
		// 像 .go, .txt, .js 这样在末尾的，大概率是文件名
		return false
	}

	return true
}

// CleanWindowTitle 清洗窗口标题，返回应用名
func CleanWindowTitle(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "Unknown"
	}

	// 0. 整体匹配检查
	if isKnownApp(raw) {
		return raw
	}

	// 1. 定义分隔符
	// 优先匹配带空格的分隔符，这能避免切分带连字符的单词（如 "Wi-Fi"）
	// 同时也支持无空格的特殊分隔符
	separators := []string{" — ", " - ", " : ", " | ", " · ", "—", "|"}

	for _, sep := range separators {
		if strings.Contains(raw, sep) {
			parts := strings.Split(raw, sep)

			// 策略：现代 GUI (Gnome/KDE/Windows) 通常格式为 "Document Title - App Name"
			// 所以优先检查最右边
			right := strings.TrimSpace(parts[len(parts)-1])
			if isLikelyAppName(right) {
				return right
			}

			// 其次检查最左边 (针对 "App Name - Document" 老式风格)
			left := strings.TrimSpace(parts[0])
			if isLikelyAppName(left) {
				return left
			}

			// 如果两边都不像（比如都有点，或者都超长），
			// 但既然发生了分割，右边作为应用名的概率通常在统计上更高。
			// 除非右边看起来明显像个版本号（比如 "1.0.0"），这里简单返回右边作为兜底。
			return right
		}
	}

	// 2. 没有分隔符的情况
	// 如果整体像应用名，就返回整体
	if isLikelyAppName(raw) {
		return raw
	}

	// 3. 实在无法识别，返回原始标题，而不是 "Unknown"
	// 这样至少用户能看到一些信息，而不是丢失所有上下文
	return raw
}

// FormatAppClass 格式化应用 Class Name
func FormatAppClass(name string) string {
	name = strings.TrimSpace(name)
	// 移除常见的反向域名并取最后一部分
	if idx := strings.LastIndex(name, "."); idx != -1 {
		name = name[idx+1:]
	}

	// 替换连字符和下划线为空格
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")

	// 首字母大写 (Title Case)
	// strings.Title 已废弃，用简单的实现
	words := strings.Fields(name)
	for i, w := range words {
		if len(w) > 0 {
			r, size := utf8.DecodeRuneInString(w)
			words[i] = string(strings.ToUpper(string(r))) + w[size:]
		}
	}
	return strings.Join(words, " ")
}
