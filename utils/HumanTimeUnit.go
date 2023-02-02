package utils

import (
	"fmt"
	"time"
)

func HumanTimeUnit(duration time.Duration) string {
	colour := "black"

	if duration > 60000000000 {
		colour = "red"
	} else if duration > 1000000000 {
		colour = "green"
	} else if duration > 1000000 {
		colour = "blue"
	}

	return fmt.Sprintf("<span style='color: %s'>%s</span>", colour, duration)
}

func FormatDuration(d time.Duration) string {
	scale := 100 * time.Second
	// look for the max scale that is smaller than d
	for scale > d {
		scale = scale / 10
	}
	return d.Round(scale / 100).String()
}
