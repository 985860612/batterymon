// Package i18n 提供 batterymon 的中英双语支持
// 默认自动探测: Windows 读系统 UI 语言, 其他平台看 LANG 环境变量; 可用 Set 强制覆盖
package i18n

import "fmt"

var lang = detect()

// Set 强制设置语言 ("zh" / "en"), 给 -lang 参数用
func Set(l string) {
	if l == "zh" || l == "en" {
		lang = l
	}
}

func IsZh() bool { return lang == "zh" }

// T 按当前语言返回对应文本
func T(zh, en string) string {
	if lang == "zh" {
		return zh
	}
	return en
}

// ETA 格式化预计时间: toFull=true 为充满还需, false 为还能用
func ETA(toFull bool, needWh, rateW float64) string {
	if rateW <= 0 {
		return "--"
	}
	mins := int(needWh / rateW * 60)
	h, m := mins/60, mins%60
	if lang == "zh" {
		if toFull {
			return fmt.Sprintf("充满还需 %d小时%02d分", h, m)
		}
		return fmt.Sprintf("还能用 %d小时%02d分", h, m)
	}
	if toFull {
		return fmt.Sprintf("to full %dh %02dm", h, m)
	}
	return fmt.Sprintf("%dh %02dm left", h, m)
}
