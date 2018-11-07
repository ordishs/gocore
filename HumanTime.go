package gocore

import (
	"fmt"
	"time"
)

// HumanTime comment
func HumanTime(d time.Duration) string {
	timeInSeconds := int64(d / time.Second)
	days := timeInSeconds / 86400
	hours := (timeInSeconds % 86400) / 3600
	minutes := ((timeInSeconds % 86400) % 3600) / 60
	seconds := (((timeInSeconds % 86400) % 3600) % 60)

	var difference string

	if days > 0 {
		difference += fmt.Sprintf("%d day", days)
		if days > 1 {
			difference += "s"
		}

		difference += fmt.Sprintf(" %d hour", hours)
		if hours > 1 {
			difference += "s"
		}

		difference += fmt.Sprintf(" %d minute", minutes)
		if minutes > 1 {
			difference += "s"
		}

		difference += fmt.Sprintf(" %d second", seconds)
		if seconds > 1 {
			difference += "s"
		}

		return difference
	}

	if hours > 0 {
		difference += fmt.Sprintf("%d hour", hours)
		if hours > 1 {
			difference += "s"
		}

		difference += fmt.Sprintf(" %d minute", minutes)
		if minutes > 1 {
			difference += "s"
		}

		difference += fmt.Sprintf(" %d second", seconds)
		if seconds > 1 {
			difference += "s"
		}

		return difference
	}

	if minutes > 0 {
		difference += fmt.Sprintf("%d minute", minutes)
		if minutes > 1 {
			difference += "s"
		}

		difference += fmt.Sprintf(" %d second", seconds)
		if seconds > 1 {
			difference += "s"
		}

		return difference
	}

	if seconds > 0 {
		difference += fmt.Sprintf("%d second", seconds)
		if seconds > 1 {
			difference += "s"
		}

		return difference
	}

	return "0 seconds"
}
