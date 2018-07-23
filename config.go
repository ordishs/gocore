package gocore

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const (
	filename = "settings.conf"
)

type configuration struct {
	confs   map[string]string
	context string
	mu      sync.RWMutex
}

var (
	c *configuration
)

// Config comment
func Config() *configuration {
	if c == nil {
		c = new(configuration)
		c.mu.Lock()
		defer c.mu.Unlock()

		// Set the context by checking the environment variable SETTINGS_CONTEXT
		env := os.Getenv("SETTINGS_CONTEXT")
		if env != "" {
			c.context = env
		}

		f, _ := filepath.Abs(filename)
		bytes, err := ioutil.ReadFile(f)
		if err != nil {
			log.Printf("Failed to read config ['%s'] - %s\n", f, err)
			os.Exit(1)
		}

		str := string(bytes)
		lines := strings.Split(str, "\n")

		c.confs = make(map[string]string, len(lines))

		for _, line := range lines {
			if len(line) > 0 {
				pos := strings.Index(line, "=")
				if pos != -1 {
					key := strings.TrimSpace(line[:pos])
					value := line[pos+1:]
					pos = strings.Index(value, "#")
					if pos != -1 {
						value = value[:pos]
					}
					value = strings.TrimSpace(value)

					c.confs[key] = value
				}
			}
		}
	}
	return c
}

// Get (key, defaultValue)
func (c *configuration) Get(key string, defaultValue ...string) (string, bool) {
	env := os.Getenv(key)
	if env != "" {
		return env, true
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	var (
		ret string
		ok  bool
	)

	if c.context != "" {
		ret, ok = c.confs[key+"."+c.context]
	}

	if ok {
		return ret, ok
	}

	ret, ok = c.confs[key]
	if ok {
		return ret, ok
	}

	if len(defaultValue) > 0 {
		ret = defaultValue[0]
	}

	return ret, false
}

func (c *configuration) GetInt(key string, defaultValue ...int) (int, bool) {
	str, ok := c.Get(key)
	if str == "" || !ok {
		if len(defaultValue) > 0 {
			return defaultValue[0], false
		}
		return 0, false
	}

	i, err := strconv.Atoi(str)
	if err != nil {
		return 0, false
	}
	return i, ok
}

func (c *configuration) Stats() string {
	out := "\nSETTINGS_CONTEXT\n----------------\n"

	if c.context != "" {
		out = out + c.context
	} else {
		out = out + "Not set"
	}

	out = out + "\n\nSETTINGS\n--------\n"
	// Get a list of keys that do not have the SESSION_CONTEXT at the end
	keys := make([]string, 0)
	context := "." + c.context

	for key := range c.confs {
		if !strings.HasSuffix(key, context) {
			keys = append(keys, key)
		}
	}

	// Sort the keys...
	sort.Strings(keys)

	// Now walk through the keys and look them up
	for _, k := range keys {
		v, _ := c.Get(k)
		out = out + fmt.Sprintf("%s=%s\n", k, v)
	}

	return out
}
