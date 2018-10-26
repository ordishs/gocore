package main

import (
	"time"

	"./anotherpackage"
	"./subpackage"

	"github.com/ordishs/gocore"
)

var logger = gocore.Log("TestPackage")

func main() {

	go func() {
		ticker := time.NewTicker(3 * time.Second)
		for range ticker.C {
			logger.Infof("This is a INFO with %s", "Args")
		}
	}()

	go func() {
		ticker2 := time.NewTicker(2 * time.Second)
		for range ticker2.C {
			anotherpackage.RunMe()
		}
	}()

	go func() {
		ticker3 := time.NewTicker(3 * time.Second)
		for range ticker3.C {
			subpackage.RunMe()
		}
	}()

	waitCh := make(chan bool)
	<-waitCh
}
