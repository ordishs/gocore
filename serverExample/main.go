package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/ordishs/gocore"
)

func main() {

	s := gocore.NewStat("something")
	s.AddTime(time.Now().UTC().UnixNano())
	s.AddTime(time.Now().UTC().UnixNano())
	s.AddTime(time.Now().UTC().UnixNano())
	s.AddTime(time.Now().UTC().UnixNano())
	s.AddTime(time.Now().UTC().UnixNano())
	s.AddTime(time.Now().UTC().UnixNano())

	g := gocore.NewStat("else")
	g.AddTime(time.Now().UTC().UnixNano())
	g.AddTime(time.Now().UTC().UnixNano())

	go func() {
		ticker := time.NewTicker(time.Millisecond * 100)

		for range ticker.C {
			key := fmt.Sprintf("stat_%d", rand.Intn(10))
			h := gocore.NewStat(key)
			h.AddTime(time.Now().UTC().UnixNano() - 1)
		}
	}()
	gocore.StartStatsServer("localhost:9001")

}
