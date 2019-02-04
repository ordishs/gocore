package main

import (
	"log"
	"os"
	"os/signal"

	"github.com/ordishs/gocore"
)

var (
	something, _ = gocore.Config().Get("something")
)

var (
	value            string
	valueWithDefault string
	numberValue      int
)

func init() {
	stats := gocore.Config().Stats()
	log.Printf("STATS\n%s\nVERSION\n-------\n%s (%s)\n\n", stats, version, commit)

	value, _ = gocore.Config().Get("key")
	valueWithDefault, _ = gocore.Config().Get("anotherKey", "default")
	numberValue, _ = gocore.Config().GetInt("number")
}

func main() {
	// setup signal catching
	signalChan := make(chan os.Signal, 1)

	signal.Notify(signalChan, os.Interrupt)

	go func() {
		s := <-signalChan

		log.Printf("Received signal: %s", s)
		appCleanup()
		os.Exit(1)
	}()

	start()
}

func appCleanup() {
	log.Println("Example shutting down...")
}

func start() {
	waitCh := make(chan bool)
	<-waitCh
}
