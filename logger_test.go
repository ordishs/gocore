package gocore

import (
	"testing"
)

func TestNewLogger(t *testing.T) {
	logger := NewLogger("Gocore", "Test", true)

	if logger.packageName != "GOCORE" {
		t.Errorf("Expected 'GOCORE' but got '%s'", logger.packageName)
	}

	if logger.serviceName != "TEST" {
		t.Errorf("Expected 'TEST' but got '%s'", logger.serviceName)
	}

}
