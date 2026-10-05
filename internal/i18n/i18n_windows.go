//go:build windows

package i18n

import (
	"os"
	"strings"
	"syscall"
)

// detect Windows 下读用户默认 UI 语言 (GetUserDefaultUILanguage),
// LANG 环境变量可覆盖 (主要方便 SSH 会话测试)
func detect() string {
	if strings.Contains(strings.ToLower(os.Getenv("LANG")), "zh") {
		return "zh"
	}
	p := syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage")
	r, _, _ := p.Call()
	if uint16(r)&0xFF == 0x04 { // PRIMARYLANGID == LANG_CHINESE
		return "zh"
	}
	return "en"
}
