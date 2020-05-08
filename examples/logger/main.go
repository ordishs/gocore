package main

import (
	"time"

	"github.com/ordishs/gocore"
)

var logger = gocore.Log("TestPackage")

func main() {

	go func() {
		ticker := time.NewTicker(3 * time.Second)
		for range ticker.C {
			logger.Debugf("This is a DEBUG with %s", "Args")
		}
	}()

	go func() {
		ticker := time.NewTicker(3 * time.Second)
		for range ticker.C {
			logger.Infof("This is a INFO with %s", "Args")
		}
	}()

	go func() {
		ticker2 := time.NewTicker(2 * time.Second)
		for range ticker2.C {
			logger.Debugf("This is a another package log with %s", "Different Args")
		}
	}()

	go func() {
		ticker3 := time.NewTicker(3 * time.Second)
		for range ticker3.C {
			logger.Warnf("This is a Subpackage log with %s", "Args")
		}
	}()

	go func() {
		ticker3 := time.NewTicker(3 * time.Second)
		for range ticker3.C {
			logger.Errorf("This is an error log with %s", "Args")
		}
	}()

	waitCh := make(chan bool)
	<-waitCh
}
