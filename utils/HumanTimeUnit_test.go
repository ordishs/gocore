package utils

import (
	"math"
	"testing"
	"time"
)

func TestFormatting(t *testing.T) {
	for i := 0; i < 15; i++ {
		dur := time.Duration(3.455555 * math.Pow(10, float64(i)))
		t.Logf("%2d  %12v  %22s", i, dur, HumanTimeUnit(dur))
	}
}
