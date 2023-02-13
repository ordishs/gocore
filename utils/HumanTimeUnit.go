package utils

import (
	"fmt"
	"time"
)

func HumanTimeUnitWithColour(d time.Duration) (string, string) {
	remainingNanos := float64(d)

	days := int64(remainingNanos / 1e9 / 86400)
	remainingNanos -= float64(days * 1e9 * 86400)

	hours := int64(remainingNanos / 1e9 / 3600)
	remainingNanos -= float64(hours * 1e9 * 3600)

	minutes := int64(remainingNanos / 1e9 / 60)
	remainingNanos -= float64(minutes * 1e9 * 60)

	seconds := int64(remainingNanos / 1e9)

	var colour string
	var str string

	if days > 0 {
		colour = "red"
		str = fmt.Sprintf("%dd%dh%dm%ds", days, hours, minutes, seconds)
	} else if hours > 0 {
		colour = "red"
		str = fmt.Sprintf("%dh%dm%ds", hours, minutes, seconds)
	} else if minutes > 0 {
		colour = "orange"
		str = fmt.Sprintf("%dm%ds", minutes, seconds)
	} else if remainingNanos > 1e9 {
		colour = "blue"
		str = fmt.Sprintf("%.2fs", remainingNanos/1e9)
	} else if remainingNanos > 1e6 {
		colour = "green"
		str = fmt.Sprintf("%.2fms", remainingNanos/1e6)
	} else if remainingNanos > 1e3 {
		colour = "black"
		str = fmt.Sprintf("%.2fµs", remainingNanos/1e3)
	} else {
		colour = "grey"
		str = fmt.Sprintf("%dns", int64(remainingNanos))
	}

	return str, colour
}

func HumanTimeUnit(d time.Duration) string {
	str, _ := HumanTimeUnitWithColour(d)
	return str
}

func HumanTimeUnitHTML(d time.Duration) string {
	str, colour := HumanTimeUnitWithColour(d)
	return fmt.Sprintf("<span style='color: %s'>%s</span>", colour, str)
}
