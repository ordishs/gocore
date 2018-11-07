package gocore

import (
	"fmt"
	"testing"
	"time"
)

func TestNewStat(t *testing.T) {
	s := NewStat("me")
	now := time.Now().UTC().UnixNano()
	s.AddTime(now - 2)
	s.AddTime(now - 1)
	NewStat("you")

	for k, v := range s.getRoot().children {
		fmt.Println(k, v)

	}
}
