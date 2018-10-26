package subpackage

import (
	"github.com/ordishs/gocore"
)

var logger = gocore.Log("TestPackage")

// RunMe Comment
func RunMe() {
	logger.Warnf("This is a Subpackage log with %s", "Args")
}
