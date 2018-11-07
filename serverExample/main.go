package main

import (
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

	h := gocore.NewStat("else")
	h.AddTime(time.Now().UTC().UnixNano())

	gocore.StartServer()

}
