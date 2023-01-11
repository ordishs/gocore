package gocore

import (
	"testing"
)

func TestLogger(t *testing.T) {
	logger := Log("TEST")

	logger.Infof("Hello world")
	logger.Errorf("Hello world")
}
