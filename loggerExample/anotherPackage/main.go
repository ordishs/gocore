package anotherpackage

import (
	"github.com/ordishs/gocore"
)

var logger = gocore.Log("TestPackage")

// RunMe Comment
func RunMe() {
	logger.Debugf("This is a another package log with %s", "Different Args")
}
