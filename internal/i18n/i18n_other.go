//go:build !windows

package i18n

import (
	"os"
	"strings"
)

func detect() string {
	for _, v := range []string{os.Getenv("LC_ALL"), os.Getenv("LC_CTYPE"), os.Getenv("LANG")} {
		if strings.Contains(strings.ToLower(v), "zh") {
			return "zh"
		}
	}
	return "en"
}
