package snapshot

import (
	"strings"
	"time"

	"github.com/fifi/internal/kst"
)

const kstLayout = "2006-01-02 15:04 MST"

func normalizeDate(value string) (string, time.Time) {
	loc := kst.Location
	value = strings.TrimSpace(strings.ReplaceAll(value, "-", ""))
	if len(value) == 8 {
		if t, err := time.ParseInLocation("20060102", value, loc); err == nil {
			return value, time.Date(t.Year(), t.Month(), t.Day(), 15, 30, 0, 0, loc)
		}
	}
	now := time.Now().In(loc)
	return now.Format("20060102"), now
}
