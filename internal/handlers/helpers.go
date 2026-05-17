package handlers

import (
	"fmt"
	"html/template"
	"math"
	"strings"
	"time"
)

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"formatBytes":   formatBytes,
		"formatRatio":   formatRatio,
		"formatBonus":   formatBonus,
		"ratioClass":    ratioClass,
		"staleAge":      staleAge,
		"isStale":       isStale,
		"prowlarrBadge": prowlarrBadge,
		"replace":       strings.ReplaceAll,
		"now":           func() time.Time { return time.Now() },
	}
}

func formatBytes(n int64) string {
	if n == 0 {
		return "—"
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	labels := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	val := float64(n) / float64(div)
	if val >= 100 {
		return fmt.Sprintf("%.0f %s", val, labels[exp])
	}
	return fmt.Sprintf("%.1f %s", val, labels[exp])
}

func formatRatio(f float64) string {
	if f == 0 {
		return "—"
	}
	return fmt.Sprintf("%.2f", f)
}

func formatBonus(f float64) string {
	if f == 0 {
		return "—"
	}
	if f >= 1_000_000 {
		return fmt.Sprintf("%.1fM", f/1_000_000)
	}
	if f >= 1000 {
		// add comma separators
		n := int64(f)
		return addCommas(n)
	}
	return fmt.Sprintf("%.0f", f)
}

func addCommas(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return addCommas(n/1000) + "," + fmt.Sprintf("%03d", n%1000)
}

func ratioClass(f float64) string {
	switch {
	case f == 0:
		return "ratio-none"
	case f < 0.5:
		return "ratio-danger"
	case f < 1.0:
		return "ratio-warn"
	case f < 2.0:
		return "ratio-ok"
	default:
		return "ratio-high"
	}
}

func staleAge(t *time.Time) string {
	if t == nil {
		return "never synced"
	}
	d := time.Since(*t)
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	}
	m := int(math.Floor(d.Minutes()))
	s := int(d.Seconds()) % 60
	if m < 60 {
		return fmt.Sprintf("%dm %02ds ago", m, s)
	}
	h := m / 60
	m = m % 60
	return fmt.Sprintf("%dh %02dm ago", h, m)
}

func isStale(t *time.Time) bool {
	if t == nil {
		return false
	}
	return time.Since(*t) > 10*time.Minute
}

func prowlarrBadge(id int, enabled bool) string {
	if id == 0 {
		return "not-added"
	}
	if enabled {
		return "enabled"
	}
	return "disabled"
}
