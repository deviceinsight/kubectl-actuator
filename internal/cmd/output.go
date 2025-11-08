package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

// Output format constants
const (
	OutputFormatWide = "wide"
	OutputFormatName = "name"
)

// newTableWriter creates a consistently configured tabwriter for table output.
func newTableWriter() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
}

// formatDurationCompact formats a duration as a compact string like "1h2m3s".
func formatDurationCompact(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	secs := int64((d + time.Second/2) / time.Second)
	h := secs / 3600
	m := (secs % 3600) / 60
	s := secs % 60

	var b strings.Builder
	if h > 0 {
		fmt.Fprintf(&b, "%dh", h)
	}
	if m > 0 {
		fmt.Fprintf(&b, "%dm", m)
	}
	if s > 0 || b.Len() == 0 {
		fmt.Fprintf(&b, "%ds", s)
	}
	return b.String()
}

// formatSecondsHuman formats seconds as a human-readable string with appropriate units.
func formatSecondsHuman(seconds float64) string {
	if seconds < 0.001 {
		return fmt.Sprintf("%.2f µs", seconds*1000000)
	}
	if seconds < 1 {
		return fmt.Sprintf("%.2f ms", seconds*1000)
	}
	if seconds < 60 {
		return fmt.Sprintf("%.2f s", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%.2f m", seconds/60)
	}
	return fmt.Sprintf("%.2f h", seconds/3600)
}

// formatBytesHuman formats bytes as a human-readable string (e.g., "1.5 KB", "2.3 MB").
func formatBytesHuman(bytes float64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%.0f B", bytes)
	}
	div, exp := float64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", bytes/div, "KMGTPE"[exp])
}

// capitalizeFirst returns s with the first character converted to uppercase.
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
