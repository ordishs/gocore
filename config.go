package gocore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
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
	c           *Configuration
	once        sync.Once
	packageName atomic.Value
)

// Config comment
func Config() *Configuration {
	once.Do(func() {
		c = new(Configuration)

		// Set the context by checking the environment variable SETTINGS_CONTEXT
		env := os.Getenv("SETTINGS_CONTEXT")
		if env != "" {
			c.context = env
		}

		f, _ := filepath.Abs(filename)
		bytes, err := ioutil.ReadFile(f)

		for err != nil && f != "/"+filename {

			dir := filepath.Dir(f)
			dir = filepath.Join(dir, "..")

			f, _ = filepath.Abs(filepath.Join(dir, filename))
			bytes, err = ioutil.ReadFile(f)
		}

		if err != nil {
			log.Printf("Failed to read config ['%s'] - %s\n", f, err)
			os.Exit(1)
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

					// As an edge case, remove the first and last characters
					// if they are both double quotes
					if len(value) > 2 && value[0] == '"' && value[len(value)-1] == '"' {
						value = value[1 : len(value)-1]
					}

					c.confs[key] = value
				}
			}
		}

		advertisingURL, _ := c.Get("advertisingURL")

		if advertisingURL != "" {
			advertisingInterval, _ := c.Get("advertisingInterval", "1m")
			logInfof("Advertising service every %s to %q\n", advertisingInterval, advertisingURL)

			interval, err := time.ParseDuration(advertisingInterval)
			if err != nil {
				interval = time.Duration(1 * time.Minute)
			}

			startTime := time.Now().UTC().Format(time.RFC3339)

			host, err := os.Hostname()
			if err != nil {
				host = "UNKNOWN"
			}

			executable := os.Args[0]

			go func() {
				time.Sleep(1 * time.Second) // Sleep for 1 second to let packageName to be set

				ticker := time.NewTicker(interval)

				type payload struct {
					Executable   string `json:"executable"`
					ServiceName  string `json:"serviceName"`
					Context      string `json:"context"`
					SettingsFile string `json:"settingsFile"`
					Host         string `json:"host"`
					StartTime    string `json:"startTime"`
				}

				for ; true; <-ticker.C {
					p, ok := packageName.Load().(string)
					if !ok {
						p = "Unknown"
					}

					j, err := json.Marshal(&payload{
						Executable:   executable,
						ServiceName:  p,
						Context:      env,
						SettingsFile: f,
						Host:         host,
						StartTime:    startTime,
					})

					if err != nil {
						logWarnf("Advertising ERROR: %v\n", err)
						continue
					}

					// log.Printf("%s\n", string(j))

					_, err = postJSON(advertisingURL, j)
					if err != nil {
						logWarnf("Advertising ERROR %v\n", err)
						continue
					}
				}
			}()
		}
	})

	return c
}

func logInfof(msg string, args ...interface{}) {
	if logger != nil {
		logger.Infof(msg, args...)
	} else {
		log.Printf(msg, args...)
	}
}

func logWarnf(msg string, args ...interface{}) {
	if logger != nil {
		logger.Warnf(msg, args...)
	} else {
		log.Printf(msg, args...)
	}
}

func postJSON(url string, j []byte) (string, error) {

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(j))
	req.Header.Set("Content-Type", "application/json")

	client := http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// fmt.Println("response Status:", resp.Status)
	// fmt.Println("response Headers:", resp.Header)
	body, err := ioutil.ReadAll(resp.Body)
	// fmt.Println("response Body:", string(body))
	if err != nil {
		return "", err
	}

	return string(body), err
}

// SetPackageName function
func SetPackageName(name string) {
	packageName.Store(name)
}

// Set an item in the config
func (c *Configuration) Set(key string, value string) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	oldValue := c.confs[key]
	c.confs[key] = value
	return oldValue
}

// Unset removes an item from the config
func (c *Configuration) Unset(key string) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	oldValue := c.confs[key]
	delete(c.confs, key)
	return oldValue
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
func (c *Configuration) GetBool(key string, defaultValue ...bool) bool {
	str, ok := c.Get(key)
	if str == "" || !ok {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
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
