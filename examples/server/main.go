package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/ordishs/gocore"
)

func main() {
	s := gocore.NewStat("something")
	s.AddTime(time.Now().UTC())
	s.AddTime(time.Now().UTC())
	s.AddTime(time.Now().UTC())
	s.AddTime(time.Now().UTC())
	s.AddTime(time.Now().UTC())
	s.AddTime(time.Now().UTC())

	l := gocore.NewStat("long")
	l.AddTime(time.Now().UTC().Add(-1 * time.Minute))
	l.AddTime(time.Now().UTC().Add(-1 * time.Minute))

	g := gocore.NewStat("else")
	time.Sleep(time.Millisecond * 100)
	g.AddTime(time.Now().UTC())
	g.AddTime(time.Now().UTC())

	h := g.NewStat("hello")
	h.AddTime(time.Now().UTC())

	j := h.NewStat("Another")
	j.AddTime(time.Now().UTC())

	go func() {
		ticker := time.NewTicker(time.Millisecond * 100)

		for range ticker.C {
			start := time.Now().UTC().Add(-1 * time.Nanosecond)
			key := fmt.Sprintf("stat_%d", rand.Intn(10))
			h := gocore.NewStat(key)
			h.AddTime(start)
		}
	}()
	gocore.StartStatsServer("localhost:9001")

}
