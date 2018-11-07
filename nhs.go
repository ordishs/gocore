package gocore

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

var startTime = time.Now().Unix()

// NHSConfig service name, service type, health service url, interval, custom report callback
type NHSConfig struct {
	Name            string
	Type            string
	URL             string
	IntervalSeconds int
	Callback        func() NHSReport
}

//Dependency dependant service statuses
type Dependency struct {
	Service string `json:"service"`
	Status  string `json:"status"`
}

// NHSReport custom health status report
type NHSReport struct {
	Dependencies []Dependency `json:"dependencies"`
	Status       string       `json:"status"`
}

type healthReport struct {
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	StartTime int64     `json:"startTime"`
	Report    NHSReport `json:"report"`
}

func postHealthReport(url string, report []byte) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(report))
	if err != nil {
		log.Printf("ERROR: Could not build request for %s. %+v", url, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("ERROR: Could not send health report to %s. %+v", url, err)
		return
	}
	defer resp.Body.Close()
}

// Register Set a custom health status
func Register(config NHSConfig) {

	if config.IntervalSeconds == 0 {
		log.Println("WARN: intervalSeconds not set. Using default of 5 seconds")
		config.IntervalSeconds = 5
	}

	if config.Name == "" {
		log.Println("ERROR: name (the unique name of your service) not set")
		return
	}

	if config.Type == "" {
		log.Println("WARN: service type not set")
		return
	}

	if config.URL == "" {
		log.Println("ERROR: health service url not set")
		return
	}

	if config.Callback == nil {
		log.Println("ERROR: no custom health function")
		return
	}

	health := healthReport{
		Name:      config.Name,
		Type:      config.Type,
		StartTime: startTime,
	}

	go func() {
		for range time.Tick(time.Duration(config.IntervalSeconds) * time.Second) {
			if config.Callback != nil {
				health.Report = config.Callback()
			}

			healthJSON, _ := json.Marshal(&health)
			// log.Printf("%+v", string(healthJSON))
			postHealthReport(config.URL, healthJSON)
		}
	}()
}
