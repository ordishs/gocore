package utils

import (
	"fmt"
	"time"
)

func HumanTimeUnit(duration time.Duration) string {
	colour := "black"

	if duration > 60000000000 {
		colour = "red"
	}

	if duration > 1000000000 {
		colour = "green"
	}

	if duration > 1000000 {
		colour = "blue"
	}

	return fmt.Sprintf("<span style='color: %s'>%s</span>", colour, duration)
}
