package gocore

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
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

// Configuration comment
type Configuration struct {
	confs   map[string]string
	context string
	mu      sync.RWMutex
}

var (
	c *Configuration
)

// GetOutboundIP comment
func GetOutboundIP() (ip net.IP, err error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	ip = localAddr.IP

	return
}

// Config comment
func Config() *Configuration {
	if c == nil {
		c = new(Configuration)
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
			f, _ := filepath.Abs(filepath.Join("..", filename))
			bytes, err = ioutil.ReadFile(f)
			if err != nil {
				log.Printf("Failed to read config ['%s'] - %s\n", f, err)
				os.Exit(1)
			}
		}

		str := string(bytes)
		lines := strings.Split(str, "\n")

		c.confs = make(map[string]string, 0)

		for _, line := range lines {
			if len(line) > 0 {
				line = strings.Split(line, "#")[0]
				pos := strings.Index(line, "=")
				if pos != -1 {
					key := strings.TrimSpace(line[:pos])
					value := line[pos+1:]
					value = strings.TrimSpace(value)

					c.confs[key] = value
				}
			}
		}
	}
	return c
}

// Get (key, defaultValue)
func (c *Configuration) Get(key string, defaultValue ...string) (string, bool) {
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

	// Start with a copy of the context, i.e. "live.eupriv"
	k := key
	if c.context != "" {
		k += "." + c.context
	}
	for !ok {
		ret, ok = c.confs[k]
		if ok {
			break
		} else {
			pos := strings.LastIndex(k, ".")
			if pos == -1 {
				break
			}
			k = k[:pos]
		}
	}

	if ok {
		return ret, ok
	}

	if len(defaultValue) > 0 {
		ret = defaultValue[0]
	}

	return ret, false
}

// GetInt comment
func (c *Configuration) GetInt(key string, defaultValue ...int) (int, bool) {
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

// GetBool comment
func (c *Configuration) GetBool(key string) bool {
	str, ok := c.Get(key)
	if str == "" || !ok {
		return false
	}

	i, err := strconv.ParseBool(str)
	if err != nil {
		return false
	}
	return i
}

// Stats comment
func (c *Configuration) Stats() string {
	out := "\nSETTINGS_CONTEXT\n----------------\n"

	if c.context != "" {
		out = out + c.context
	} else {
		out = out + "Not set"
	}

	out = out + "\n\nSETTINGS\n--------\n"
	// Get a list of keys that do not have the SESSION_CONTEXT at the end
	keysMap := make(map[string]struct{}, 0)
	for item := range c.confs {
		keysMap[strings.Split(item, ".")[0]] = struct{}{}
	}

	// Sort the keys...
	keysArr := make([]string, 0)
	for k := range keysMap {
		keysArr = append(keysArr, k)
	}
	sort.Strings(keysArr)

	// Now walk through the keys and look them up
	for _, k := range keysArr {
		v, _ := c.Get(k)
		out = out + fmt.Sprintf("%s=%s\n", k, v)
	}

	return out
}
