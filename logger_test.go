package gocore

import (
	"reflect"
	"testing"
)

func TestLogger(t *testing.T) {
	logger := Log("TEST")

	logger.Infof("Hello world")
	logger.Errorf("Hello world")
}

func TestLog(t *testing.T) {
	type args struct {
		packageName string
	}
	tests := []struct {
		name string
		args args
		want *Logger
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Log(tt.args.packageName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Log() = %v, want %v", got, tt.want)
			}
		})
	}
}
