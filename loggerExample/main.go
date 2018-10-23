package main

import (
	"time"

	"github.com/ordishs/gocore"
)

var logger = gocore.NewLogger("test", "main", true)

func main() {
	ticker := time.NewTicker(1 * time.Second)
	for range ticker.C {
		logger.Debugf("Debug")
	}
}
