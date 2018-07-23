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

func healthReport() gocore.NHSReport {
	var dependencies [2]gocore.Dependency
	dependencies[0].Service = "Service 1"
	dependencies[0].Status = "AMBER"
	dependencies[1].Service = "Service 2"
	dependencies[1].Status = "GREEN"
	return gocore.NHSReport{
		Dependencies: dependencies[:],
		Status:       "GREEN",
	}
}

func init() {
	stats := gocore.Config().Stats()
	log.Printf("STATS\n%s\nVERSION\n-------\n%s (%s)\n\n", stats, version, commit)

	nhsURL, _ := gocore.Config().Get("nhsUrl")

	config := gocore.NHSConfig{
		Name:     "Example Microservice Instance 1",
		Type:     "Example MicroService",
		URL:      nhsURL,
		Callback: healthReport,
	}
	gocore.Register(config)

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
