package gocore

import (
	"fmt"
)

// HumanTimeUnit comment
func HumanTimeUnit(nanos int64) string {
	if nanos > 60000000000 {
		return fmt.Sprintf("<span style='color: red'>%.2fm</span>", float64(nanos)/60000000000.0)
	}

	if nanos > 1000000000 {
		return fmt.Sprintf("<span style='color: green'>%.2fs</span>", float64(nanos)/1000000000.0)
	}

	if nanos > 1000000 {
		return fmt.Sprintf("<span style='color: blue'>%.2fms</span>", float64(nanos)/1000000.0)
	}

	return fmt.Sprintf("%dns", nanos)
}
