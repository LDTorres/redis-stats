package redisstats

import "fmt"

func formatBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}

	div, exp := uint64(unit), 0
	for n := value / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(div), "KMGTPE"[exp])
}

func formatBytesSigned(value int64) string {
	if value == 0 {
		return "0 B"
	}

	sign := "+"
	if value < 0 {
		sign = "-"
		value = -value
	}

	return sign + formatBytes(uint64(value))
}
