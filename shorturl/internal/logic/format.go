package logic

import (
	"strings"
	"time"
)

const displayDateTimeLayout = "2006-01-02 15:04:05"

func formatLocalDateTime(t time.Time) string {
	return t.In(time.Local).Format(displayDateTimeLayout)
}

func normalizeCategoryDisplay(s string) string {
	if strings.TrimSpace(s) == "" {
		return "其他"
	}
	return strings.TrimSpace(s)
}
