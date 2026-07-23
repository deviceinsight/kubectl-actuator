package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"

	"github.com/spf13/cobra"
)

const (
	OutputFormatWide = "wide"
	OutputFormatName = "name"
	OutputFormatJSON = "json"
	OutputFormatYAML = "yaml"
)

// addOutputFlag registers the -o/--output flag; the allowed formats double
// as its completion suggestions, so tab never falls back to filenames.
func addOutputFlag(cmd *cobra.Command, target *string, defaultValue string, formats ...string) {
	cmd.Flags().StringVarP(target, "output", "o", defaultValue, "Output format. One of: "+strings.Join(formats, ", "))
	_ = cmd.RegisterFlagCompletionFunc("output", cobra.FixedCompletions(formats, cobra.ShellCompDirectiveNoFileComp))
}

// markNoFileFlags disables filename completion for flags whose values can
// never be files, such as filters and numbers; wrong suggestions are worse
// than none.
func markNoFileFlags(cmd *cobra.Command, names ...string) {
	for _, name := range names {
		_ = cmd.RegisterFlagCompletionFunc(name, cobra.NoFileCompletions)
	}
}

// isStructuredOutput reports whether the format produces a machine-readable
// document (json/yaml) instead of tables.
func isStructuredOutput(format string) bool {
	return format == OutputFormatJSON || format == OutputFormatYAML
}

func validateOutputFormat(output string, allowed ...string) error {
	if output == "" {
		return nil
	}
	for _, a := range allowed {
		if output == a {
			return nil
		}
	}
	return fmt.Errorf("invalid output format %q, must be one of: %s", output, strings.Join(allowed, ", "))
}

// newTableWriter creates a consistently configured tabwriter for table output.
func newTableWriter() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
}

// matchesFilter reports whether name matches the filter using the tool-wide
// filter semantics: empty filter matches everything, otherwise case-insensitive
// substring match.
func matchesFilter(name, filter string) bool {
	if filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(name), strings.ToLower(filter))
}

func hasPrefixIgnoreCase(s, prefix string) bool {
	return strings.HasPrefix(strings.ToLower(s), strings.ToLower(prefix))
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

// formatSecondsHuman formats seconds with an auto-scaled unit, e.g. "350.00 ms", "1.25 m".
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

func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// titleCaseKey turns a camelCase JSON key into a spaced title:
// "podName" -> "Pod Name", "hostIP" -> "Host IP".
func titleCaseKey(key string) string {
	var b strings.Builder
	runes := []rune(key)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) && !unicode.IsUpper(runes[i-1]) {
			b.WriteRune(' ')
		}
		b.WriteRune(r)
	}
	return capitalizeFirst(b.String())
}

// truncateString cuts s to maxLen runes, marking the cut with an ellipsis.
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-1]) + "…"
}

func escapeValue(s string) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}
