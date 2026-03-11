package parser

import (
	"fmt"
	"time"
)

func ParseDateTime(value string) (time.Time, error) {
	formats := []string{
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, value); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported datetime format: %s", value)
}

func ParseDate(value string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		"2006/01/02",
		"02/01/2006",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, value); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported date format: %s", value)
}
