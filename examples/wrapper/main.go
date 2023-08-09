package main

import (
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/ordishs/gocore"
)

func init() {
	stats := gocore.Config().Stats()
	log.Printf("STATS\n%s\nVERSION\n-------\n%s (%s)\n\n", stats, version, commit)
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
	secret, _ := gocore.Config().Get("secret")
	log.Printf("secret=%s", secret)

	waitCh := make(chan bool)

	go func() {
		time.Sleep(2 * time.Second)
		waitCh <- true
	}()

	<-waitCh
}
