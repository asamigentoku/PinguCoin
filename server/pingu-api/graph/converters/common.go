package converters

import "time"

func FormatTimestamp(timestamp time.Time) string {
	return timestamp.Format(time.RFC3339)
}

func StringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
