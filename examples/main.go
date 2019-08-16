package main

import (
	"fmt"

	"github.com/ordishs/gocore"
)

func main() {
	name, _ := gocore.Config().Get("name")
	fmt.Println(name)
}
