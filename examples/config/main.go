package main

import (
	"fmt"

	"github.com/ordishs/gocore"
)

func main() {
	stats := gocore.Config().Stats()

	fmt.Printf("STATS\n%s\n\n\n", stats)

	v, _ := gocore.Config().Get("embedded")
	fmt.Printf("embedded: %s\n", v)

	gocore.Config().Set("name", "bob")

	v, _ = gocore.Config().Get("embedded")
	fmt.Printf("embedded: %s\n", v)

	v, _ = gocore.Config().Get("embedded2")
	fmt.Printf("embedded2: %s\n", v)

	val, ok := gocore.Config().Get("key0")
	fmt.Printf("key0 (no default): %s, %v\n", val, ok)

	val, ok = gocore.Config().Get("key0", "")
	fmt.Printf("key0 (empty default): %s, %v\n", val, ok)

	val, ok = gocore.Config().Get("key0", "VALUE")
	fmt.Printf("key0 (supplied default): %s, %v\n", val, ok)

	fmt.Println()

	val, ok = gocore.Config().Get("key1")
	fmt.Printf("key1 (no default): %s, %v\n", val, ok)

	val, ok = gocore.Config().Get("key1", "")
	fmt.Printf("key1 (empty default): %s, %v\n", val, ok)

	val, ok = gocore.Config().Get("key1", "value")
	fmt.Printf("key1 (supplied default): %s, %v\n", val, ok)

	fmt.Println()

	val, ok = gocore.Config().Get("key2")
	fmt.Printf("key2 (no default): %s, %v\n", val, ok)

	val, ok = gocore.Config().Get("key2", "")
	fmt.Printf("key2 (empty default): %s, %v\n", val, ok)

	val, ok = gocore.Config().Get("key2", "value")
	fmt.Printf("key2 (supplied default): %s, %v\n", val, ok)

}
