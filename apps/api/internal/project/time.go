package project

import (
	"fmt"
	"time"
)

const timeFormatRFC3339Nano = time.RFC3339Nano

func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(timeFormatRFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse cursor updated_at: %w", err)
	}
	return parsed.UTC(), nil
}
