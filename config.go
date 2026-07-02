package gocore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"text/tabwriter"
	"time"

	"github.com/joho/godotenv"
	"github.com/ordishs/gocore/utils"
)

// Configuration comment
type Configuration struct {
	confs      map[string]string
	context    string
	app        string
	requests   map[string]*requestRecord
	rmu        sync.RWMutex
	mu         sync.RWMutex
	listeners  []SettingsListener
	listenerMu sync.RWMutex
}

var (
	c                   *Configuration // This is the default config
	once                sync.Once
	alternativeConfigs  map[string]*Configuration
	configsMu           sync.RWMutex
	packageName         atomic.Value
	address             atomic.Value
	version             atomic.Value
	commit              atomic.Value
	appMu               sync.RWMutex
	appPayloadFunctions map[string]func() interface{}
)

var reEHE = regexp.MustCompile(`(\*EHE\*[a-zA-Z0-9]+)`)

const eheMask = "********************"

func maskSecrets(value string) string {
	if strings.HasPrefix(value, "*EHE*") {
		return eheMask
	}

	return reEHE.ReplaceAllString(value, eheMask)
}

type requestRecord struct {
	Key            string
	DefaultValue   string
	HasDefault     bool
	Value          string
	Source         string
	FirstRequested time.Time
	LastRequested  time.Time
	Count          int64
}

func init() {
	packageName.Store("gocore")
	alternativeConfigs = make(map[string]*Configuration)
}

func AddAppPayloadFn(key string, fn func() interface{}) {
	appMu.Lock()
	defer appMu.Unlock()

	if appPayloadFunctions == nil {
		appPayloadFunctions = make(map[string]func() interface{})
	}

	appPayloadFunctions[key] = fn
}

// SetInfo comment
func SetInfo(name string, ver string, com string) {
	packageName.Store(name)
	version.Store(ver)
	commit.Store(com)
}

// SetAddress comment
func SetAddress(addr string) {
	address.Store(addr)
}

func GetPackageName() string {
	p, ok := packageName.Load().(string)
	if !ok {
		return "Unknown"
	}

	return p
}

func GetVersion() string {
	v, ok := version.Load().(string)
	if !ok {
		return "Unknown"
	}

	return v
}

func GetCommit() string {
	c, ok := commit.Load().(string)
	if !ok {
		return "Unknown"
	}

	return c
}

func GetAddress() string {
	a, ok := address.Load().(string)
	if !ok {
		return "Unknown"
	}

	return a
}

func processFile(m map[string]string, filename string) (string, error) {
	// Get the directory of the application binary
	exePath, err := os.Executable()
	if err != nil {
		return filename, err
	}

	binaryDir := filepath.Dir(exePath)

	// Start looking in the binary's parent-parent directory
	f, err := filepath.Abs(filepath.Join(binaryDir, filename))
	if err != nil {
		return filename, err
	}

	bytesRead, err := os.ReadFile(f)

	// Try parent traversal logic
	for err != nil && f != "/"+filename {
		dir := filepath.Dir(f)
		dir = filepath.Join(dir, "..") // Go up two levels
		f, err = filepath.Abs(filepath.Join(dir, filename))
		if err != nil {
			return "", err
		}

		bytesRead, err = os.ReadFile(f)
	}

	// If not found, start from the present working directory
	if err != nil {
		f, err = filepath.Abs(filename)
		if err != nil {
			return filename, err
		}
		bytesRead, err = os.ReadFile(f)

		for err != nil && f != "/"+filename {
			dir := filepath.Dir(f)
			dir = filepath.Join(dir, "..")
			f, err = filepath.Abs(filepath.Join(dir, filename))
			if err != nil {
				return "", err
			}
			bytesRead, err = os.ReadFile(f)
		}
	}

	if err != nil {
		return f, err
	}

	str := string(bytesRead)
	lines := strings.Split(str, "\n")

	for lineNum, line := range lines {
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

				oldVal, found := m[key]
				if found {
					log.Printf("INFO: %s:%d is replacing %q: %q -> %q", f, lineNum+1, key, oldVal, value)
				}
				m[key] = value
			}
		}
	}

	return f, nil
}

// Config returns a Configuration object
func Config(alternativeContext ...string) *Configuration {
	once.Do(func() {
		// Before processing settings, use dotenv to load any environment variables .env file

		// The .env file can be overridden by setting the environment variable
		// SETTINGS_ENV_FILE to the full path of the file.
		// For example:
		//   SETTINGS_ENV_FILE=/home/user/.env
		//	 SETTINGS_ENV_FILE=/home/user/.env.dev
		//   SETTINGS_ENV_FILE=/home/user/.env.prod
		//
		// If the environment variable is not set, then the default file name is .env
		envFile := os.Getenv("SETTINGS_ENV_FILE")
		if envFile == "" {
			envFile = ".env"
		}

		if _, err := os.Stat(envFile); err == nil {
			err := godotenv.Load(envFile)
			if err != nil {
				log.Print("WARN: failed to loading .env file")
			}
		}

		c = new(Configuration)

		// Set the context by checking the environment variable SETTINGS_CONTEXT
		env := os.Getenv("SETTINGS_CONTEXT")
		if env != "" {
			c.context = env
		} else {
			c.context = "dev"
		}

		// Set the application by checking the environment variable SETTINGS_APPLICATION
		app := os.Getenv("SETTINGS_APPLICATION")
		if app != "" {
			c.app = app
		}

		c.confs = make(map[string]string, 0)
		c.requests = make(map[string]*requestRecord)

		filename, err := processFile(c.confs, "settings.conf")
		if err != nil {
			if os.IsNotExist(err) {
				filename = "NOT FOUND"
				log.Println("WARN: No config file 'settings.conf'")
			} else {
				log.Printf("FATAL: Failed to read config  file '%s' - [%v]", filename, err)
				os.Exit(1)
			}
		}

		// // Load infrastructure settings
		// infraFilename, err := processFile(c.confs, "settings_infra.conf")
		// if err != nil {
		// 	if os.IsNotExist(err) {
		// 		infraFilename = "NOT FOUND"
		// 		log.Println("WARN: No infrastructure config file 'settings_infra.conf'")
		// 	} else {
		// 		log.Printf("FATAL: Failed to read infrastructure config file '%s' - [%v]", infraFilename, err)
		// 		os.Exit(1)
		// 	}
		// }

		// Load settings_test.conf, if it exists. If not, it's not a problem.
		testFilename, err := processFile(c.confs, "settings_test.conf")
		if err == nil {
			// There was a settings_test.conf loaded.  Log the filename...
			logInfof("INFO: Loaded test config file '%s'", testFilename)
		}

		// Load local overrides last
		localFilename, err := processFile(c.confs, "settings_local.conf")
		if err != nil {
			if os.IsNotExist(err) {
				localFilename = "NOT FOUND"
				log.Println("WARN: No local config file 'settings_local.conf'")
			} else {
				log.Printf("FATAL: Failed to read local config '%s' - [%v]", localFilename, err)
				os.Exit(1)
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
					Executable   string   `json:"executable"`
					ServiceName  string   `json:"serviceName"`
					Loggers      []string `json:"loggers"`
					Version      string   `json:"version"`
					Commit       string   `json:"commit"`
					Context      string   `json:"context"`
					Application  string   `json:"application"`
					SettingsFile string   `json:"settingsFile"`
					// InfraSettingsFile string                 `json:"infraSettingsFile"`
					TestSettingsFile  string                 `json:"testSettingsFile"`
					LocalSettingsFile string                 `json:"localSettingsFile"`
					Host              string                 `json:"host"`
					Address           string                 `json:"address"`
					StartTime         string                 `json:"startTime"`
					AppPayload        map[string]interface{} `json:"appPayload"`
				}

				for ; true; <-ticker.C {
					p, ok := packageName.Load().(string)
					if !ok {
						p = "Unknown"
					}

					addressStr, ok := address.Load().(string)
					if !ok {
						addressStr = "Unknown"
					}

					ver, ok := version.Load().(string)
					if !ok {
						ver = "..."
					}

					c, ok := commit.Load().(string)
					if !ok {
						c = "..."
					}

					mu.RLock()
					l := make([]string, 0)
					for name := range loggers {
						l = append(l, name)
					}
					mu.RUnlock()

					appPayloads := make(map[string]interface{})
					appMu.RLock()
					for key, fn := range appPayloadFunctions {
						appPayloads[key] = fn()
					}
					appMu.RUnlock()

					j, err := json.Marshal(&payload{
						Executable:        executable,
						ServiceName:       p,
						Loggers:           l,
						Version:           ver,
						Commit:            c,
						Context:           env,
						Application:       app,
						SettingsFile:      filename,
						LocalSettingsFile: localFilename,
						// InfraSettingsFile: infraFilename,
						TestSettingsFile: testFilename,
						Host:             host,
						Address:          addressStr,
						StartTime:        startTime,
						AppPayload:       appPayloads,
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

	if len(alternativeContext) > 0 && alternativeContext[0] != "" && alternativeContext[0] != c.context {
		// We have an alternative context and will return a modified copy
		// unless it is already in the map

		// Removed the channel wait here
		configsMu.RLock()
		ac, found := alternativeConfigs[alternativeContext[0]]
		configsMu.RUnlock()

		if found {
			return ac
		}

		configsMu.Lock()
		defer configsMu.Unlock()

		// Double check the config isn't already in the map
		if ac, found := alternativeConfigs[alternativeContext[0]]; found {
			return ac
		}

		ac = new(Configuration)
		ac.mu.Lock()
		defer ac.mu.Unlock()

		ac.context = alternativeContext[0]
		ac.app = c.app
		// Copy the confs
		ac.confs = make(map[string]string)
		for k, v := range c.confs {
			ac.confs[k] = v
		}

		ac.requests = make(map[string]*requestRecord)

		alternativeConfigs[alternativeContext[0]] = ac

		return ac
	}

	return c
}

func logInfof(msg string, args ...interface{}) {
	log.Printf(msg, args...)
}

func logWarnf(msg string, args ...interface{}) {
	log.Printf(msg, args...)
}

func postJSON(urlStr string, j []byte) (string, error) {
	if len(j) == 0 || urlStr == "" {
		logWarnf("Advertising post ERROR empty advertise URL |%v| or JSON\n", urlStr)
		return "", fmt.Errorf("Error posting JSON")
	}
	jsonBuf := bytes.NewBuffer(j)
	req, err := http.NewRequest("POST", urlStr, jsonBuf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// fmt.Println("response Status:", resp.Status)
	// fmt.Println("response Headers:", resp.Header)
	body, err := io.ReadAll(resp.Body)
	// fmt.Println("response Body:", string(body))
	if err != nil {
		return "", err
	}

	return string(body), err
}

// SettingsListener is an interface that must be implemented by any component
// that wants to be notified of settings changes
type SettingsListener interface {
	UpdateSetting(key string, value string)
}

// Set an item in the config
func (c *Configuration) Set(key string, value string) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	oldValue := c.confs[key]
	c.confs[key] = value

	// Notify all listeners of the change
	c.listenerMu.RLock()
	for _, listener := range c.listeners {
		listener.UpdateSetting(key, value)
	}
	c.listenerMu.RUnlock()

	return oldValue
}

// Unset removes an item from the config
func (c *Configuration) Unset(key string) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	oldValue := c.confs[key]
	delete(c.confs, key)

	// Notify all listeners that the setting was removed
	c.listenerMu.RLock()
	for _, listener := range c.listeners {
		listener.UpdateSetting(key, "")
	}
	c.listenerMu.RUnlock()

	return oldValue
}

func (c *Configuration) decrypt(val string) string {
	s, err := utils.DecryptSetting(val)
	if err != nil {
		return val
	}

	return s
}

func (c *Configuration) replaceVariables(value string) string {
	re := regexp.MustCompile(`(\$\{.*?\})`)
	for {
		matches := re.FindAllString(value, -1)
		if len(matches) == 0 {
			break // No more tokens to replace
		}
		for _, match := range matches {
			key := match[2 : len(match)-1]
			val, ok, _ := c.getInternal(key)
			if ok {
				val = strings.TrimPrefix(val, "*EHE*")
				value = strings.Replace(value, match, val, 1)
			} else {
				value = strings.Replace(value, match, "{UNKNOWN}", 1)
			}
		}
	}
	return value
}

func (c *Configuration) record(key string, hasDefault bool, defaultStr, value, source string) {
	masked := maskSecrets(value)
	maskedDefault := maskSecrets(defaultStr)

	now := time.Now().UTC()

	mapKey := fmt.Sprintf("%s\x00%t\x00%s", key, hasDefault, defaultStr)

	c.rmu.Lock()
	defer c.rmu.Unlock()

	if rec, found := c.requests[mapKey]; found {
		rec.Value = masked
		rec.Source = source
		rec.LastRequested = now
		rec.Count++
		return
	}

	c.requests[mapKey] = &requestRecord{
		Key:            key,
		DefaultValue:   maskedDefault,
		HasDefault:     hasDefault,
		Value:          masked,
		Source:         source,
		FirstRequested: now,
		LastRequested:  now,
		Count:          1,
	}
}

func (c *Configuration) Get(key string, defaultValue ...string) (string, bool) {
	s, ok, source := c.getInternal(key, defaultValue...)

	hasDefault := len(defaultValue) > 0
	defStr := ""
	if hasDefault {
		defStr = defaultValue[0]
	}

	c.record(key, hasDefault, defStr, s, source)

	return strings.TrimPrefix(s, "*EHE*"), ok
}

// Get (key, defaultValue)
func (c *Configuration) getInternal(key string, defaultValue ...string) (string, bool, string) {
	env, ok := os.LookupEnv(key)
	if ok {
		// Replace variables in the value
		env = c.replaceVariables(env)

		return c.decrypt(env), true, "ENV"
	}

	ret, ok, keyUsed := c.findValue(key)
	if ok {
		// Replace variables in the value
		ret = c.replaceVariables(ret)

		return c.decrypt(ret), ok, keyUsed
	}

	if len(defaultValue) > 0 {
		ret = defaultValue[0]
	}

	// Replace variables in the value
	ret = c.replaceVariables(ret)

	return c.decrypt(ret), false, "DEFAULT"
}

func (c *Configuration) findValue(key string) (ret string, ok bool, k string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Start with a copy of the context, i.e. "live.context"
	k = key
	if c.context != "" {
		k += "." + c.context
	}
	for !ok {
		if c.app != "" {
			ret, ok = c.confs[k+"."+c.app]
			if ok {
				k = k + "." + c.app
				return
			}
		}
		ret, ok = c.confs[k]
		if ok {
			return
		} else {
			pos := strings.LastIndex(k, ".")
			if pos == -1 {
				break
			}
			k = k[:pos]
		}
	}

	return
}

func (c *Configuration) GetMulti(key string, sep string, defaultValue ...[]string) ([]string, bool) {
	raw, ok, source := c.getInternal(key)
	str := strings.TrimPrefix(raw, "*EHE*")

	hasDefault := len(defaultValue) > 0
	defStr := ""
	if hasDefault {
		defStr = strings.Join(defaultValue[0], sep)
	}

	if str == "" || !ok {
		if hasDefault {
			c.record(key, hasDefault, defStr, defStr, "DEFAULT")
			return defaultValue[0], false
		}
		c.record(key, hasDefault, defStr, "", "DEFAULT")
		return []string{}, false
	}

	c.record(key, hasDefault, defStr, raw, source)

	items := strings.Split(str, sep)
	for i, item := range items {
		items[i] = strings.TrimSpace(item)
	}
	return items, ok
}

// number is used internally as a type constraint for numeric types
type number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~float32 | ~float64
}

// getNumber is a generic function to handle numeric type conversions
func getNumber[T number](c *Configuration, key string, defaultValue ...T) (T, bool, error) {
	raw, ok, source := c.getInternal(key)
	str := strings.TrimPrefix(raw, "*EHE*")

	hasDefault := len(defaultValue) > 0
	defStr := ""
	if hasDefault {
		defStr = fmt.Sprintf("%v", defaultValue[0])
	}

	if str == "" || !ok {
		if hasDefault {
			c.record(key, hasDefault, defStr, defStr, "DEFAULT")
			return defaultValue[0], false, nil
		}
		c.record(key, hasDefault, defStr, "", "DEFAULT")
		var zero T
		return zero, false, nil
	}

	c.record(key, hasDefault, defStr, raw, source)

	var result T
	var err error

	switch any(result).(type) {
	case int:
		val, e := strconv.ParseInt(str, 10, 64)
		err = e
		result = T(val)
	case int8:
		val, e := strconv.ParseInt(str, 10, 8)
		err = e
		result = T(val)
	case int16:
		val, e := strconv.ParseInt(str, 10, 16)
		err = e
		result = T(val)
	case int32:
		val, e := strconv.ParseInt(str, 10, 32)
		err = e
		result = T(val)
	case int64:
		val, e := strconv.ParseInt(str, 10, 64)
		err = e
		result = T(val)
	case uint:
		val, e := strconv.ParseUint(str, 10, 64)
		err = e
		result = T(val)
	case uint8:
		val, e := strconv.ParseUint(str, 10, 8)
		err = e
		result = T(val)
	case uint16:
		val, e := strconv.ParseUint(str, 10, 16)
		err = e
		result = T(val)
	case uint32:
		val, e := strconv.ParseUint(str, 10, 32)
		err = e
		result = T(val)
	case uint64:
		val, e := strconv.ParseUint(str, 10, 64)
		err = e
		result = T(val)
	case float32:
		val, e := strconv.ParseFloat(str, 32)
		err = e
		result = T(val)
	case float64:
		val, e := strconv.ParseFloat(str, 64)
		err = e
		result = T(val)
	}

	if err != nil {
		var zero T
		return zero, true, fmt.Errorf("failed to parse %q as %T: %w", str, zero, err)
	}

	return result, true, nil
}

func (c *Configuration) TryGetInt(key string, defaultValue ...int) (int, bool, error) {
	return getNumber(c, key, defaultValue...)
}

func (c *Configuration) GetInt(key string, defaultValue ...int) (int, bool) {
	n, found, err := getNumber(c, key, defaultValue...)
	if err != nil {
		return n, false
	}

	return n, found
}

func (c *Configuration) TryGetInt8(key string, defaultValue ...int8) (int8, bool, error) {
	return getNumber(c, key, defaultValue...)
}

func (c *Configuration) GetInt8(key string, defaultValue ...int8) (int8, bool) {
	n, found, err := getNumber(c, key, defaultValue...)
	if err != nil {
		return n, false
	}

	return n, found
}

func (c *Configuration) TryGetInt16(key string, defaultValue ...int16) (int16, bool, error) {
	return getNumber(c, key, defaultValue...)
}

func (c *Configuration) GetInt16(key string, defaultValue ...int16) (int16, bool) {
	n, found, err := getNumber(c, key, defaultValue...)
	if err != nil {
		return n, false
	}

	return n, found
}

func (c *Configuration) TryGetInt32(key string, defaultValue ...int32) (int32, bool, error) {
	return getNumber(c, key, defaultValue...)
}

func (c *Configuration) GetInt32(key string, defaultValue ...int32) (int32, bool) {
	n, found, err := getNumber(c, key, defaultValue...)
	if err != nil {
		return n, false
	}

	return n, found
}

func (c *Configuration) TryGetInt64(key string, defaultValue ...int64) (int64, bool, error) {
	return getNumber(c, key, defaultValue...)
}

func (c *Configuration) GetInt64(key string, defaultValue ...int64) (int64, bool) {
	n, found, err := getNumber(c, key, defaultValue...)
	if err != nil {
		return n, false
	}

	return n, found
}

func (c *Configuration) TryGetUint(key string, defaultValue ...uint) (uint, bool, error) {
	return getNumber(c, key, defaultValue...)
}

func (c *Configuration) GetUint(key string, defaultValue ...uint) (uint, bool) {
	n, found, err := getNumber(c, key, defaultValue...)
	if err != nil {
		return n, false
	}

	return n, found
}

func (c *Configuration) TryGetUint8(key string, defaultValue ...uint8) (uint8, bool, error) {
	return getNumber(c, key, defaultValue...)
}

func (c *Configuration) GetUint8(key string, defaultValue ...uint8) (uint8, bool) {
	n, found, err := getNumber(c, key, defaultValue...)
	if err != nil {
		return n, false
	}

	return n, found
}

func (c *Configuration) TryGetUint16(key string, defaultValue ...uint16) (uint16, bool, error) {
	return getNumber(c, key, defaultValue...)
}

func (c *Configuration) GetUint16(key string, defaultValue ...uint16) (uint16, bool) {
	n, found, err := getNumber(c, key, defaultValue...)
	if err != nil {
		return n, false
	}

	return n, found
}

func (c *Configuration) TryGetUint32(key string, defaultValue ...uint32) (uint32, bool, error) {
	return getNumber(c, key, defaultValue...)
}

func (c *Configuration) GetUint32(key string, defaultValue ...uint32) (uint32, bool) {
	n, found, err := getNumber(c, key, defaultValue...)
	if err != nil {
		return n, false
	}

	return n, found
}

func (c *Configuration) TryGetUint64(key string, defaultValue ...uint64) (uint64, bool, error) {
	return getNumber(c, key, defaultValue...)
}

func (c *Configuration) GetUint64(key string, defaultValue ...uint64) (uint64, bool) {
	n, found, err := getNumber(c, key, defaultValue...)
	if err != nil {
		return n, false
	}

	return n, found
}

func (c *Configuration) TryGetFloat32(key string, defaultValue ...float32) (float32, bool, error) {
	return getNumber(c, key, defaultValue...)
}

func (c *Configuration) GetFloat32(key string, defaultValue ...float32) (float32, bool) {
	n, found, err := getNumber(c, key, defaultValue...)
	if err != nil {
		return n, false
	}

	return n, found
}

func (c *Configuration) TryGetFloat64(key string, defaultValue ...float64) (float64, bool, error) {
	return getNumber(c, key, defaultValue...)
}

func (c *Configuration) GetFloat64(key string, defaultValue ...float64) (float64, bool) {
	n, found, err := getNumber(c, key, defaultValue...)
	if err != nil {
		return n, false
	}

	return n, found
}

func (c *Configuration) GetBool(key string, defaultValue ...bool) bool {
	raw, ok, source := c.getInternal(key)
	str := strings.TrimPrefix(raw, "*EHE*")

	hasDefault := len(defaultValue) > 0
	defStr := ""
	if hasDefault {
		defStr = strconv.FormatBool(defaultValue[0])
	}

	if str == "" || !ok {
		if hasDefault {
			c.record(key, hasDefault, defStr, defStr, "DEFAULT")
			return defaultValue[0]
		}
		c.record(key, hasDefault, defStr, "", "DEFAULT")
		return false
	}

	c.record(key, hasDefault, defStr, raw, source)

	i, err := strconv.ParseBool(str)
	if err != nil {
		return false
	}

	return i
}

func (c *Configuration) GetDuration(key string, defaultValue ...time.Duration) (time.Duration, error, bool) {
	raw, ok, source := c.getInternal(key)
	str := strings.TrimPrefix(raw, "*EHE*")

	hasDefault := len(defaultValue) > 0
	defStr := ""
	if hasDefault {
		defStr = defaultValue[0].String()
	}

	if str == "" || !ok {
		if hasDefault {
			c.record(key, hasDefault, defStr, defStr, "DEFAULT")
			return defaultValue[0], nil, false
		}
		c.record(key, hasDefault, defStr, "", "DEFAULT")
		return 0, nil, false
	}

	c.record(key, hasDefault, defStr, raw, source)

	d, err := time.ParseDuration(str)
	if err != nil {
		return 0, err, false
	}

	return d, nil, ok
}

func (c *Configuration) GetURL(key string, defaultValue ...string) (*url.URL, error, bool) {
	raw, ok, source := c.getInternal(key)
	str := strings.TrimPrefix(raw, "*EHE*")

	hasDefault := len(defaultValue) > 0
	defStr := ""
	if hasDefault {
		defStr = defaultValue[0]
	}

	if str == "" || !ok {
		if hasDefault {
			str = defaultValue[0]
			ok = false
			c.record(key, hasDefault, defStr, str, "DEFAULT")
		} else {
			c.record(key, hasDefault, defStr, "", "DEFAULT")
			return nil, errors.New("URL is missing"), false
		}
	} else {
		c.record(key, hasDefault, defStr, str, source)
	}

	ehes := reEHE.FindAllString(str, -1)

	for _, ehe := range ehes {
		decrypted, err := utils.DecryptSetting(ehe)
		if err != nil {
			continue
		}
		decrypted = strings.TrimPrefix(decrypted, "*EHE*")
		str = strings.Replace(str, ehe, decrypted, 1)
	}

	u, err := url.ParseRequestURI(str)
	if err != nil {
		return nil, err, false
	}

	return u, nil, ok
}

func (c *Configuration) GetAll() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	m := make(map[string]string, 0)

	m["_SETTINGS_CONTEXT"] = c.context

	for k, v := range c.confs {
		// Check if the key has a value in the environment
		if envVal, ok := os.LookupEnv(k); ok {
			m[k] = envVal
		} else {
			m[k] = v
		}
	}

	return m
}

func (c *Configuration) requestedSnapshot() []requestRecord {
	c.rmu.RLock()
	defer c.rmu.RUnlock()

	rows := make([]requestRecord, 0, len(c.requests))
	for _, rec := range c.requests {
		rows = append(rows, *rec)
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Key != rows[j].Key {
			return rows[i].Key < rows[j].Key
		}
		if rows[i].HasDefault != rows[j].HasDefault {
			return !rows[i].HasDefault
		}
		return rows[i].DefaultValue < rows[j].DefaultValue
	})

	return rows
}

func (c *Configuration) Requested() string {
	rows := c.requestedSnapshot()

	var builder strings.Builder
	w := tabwriter.NewWriter(&builder, 0, 0, 2, ' ', 0)

	fmt.Fprintln(w, "KEY\tVALUE\tSOURCE\tDEFAULT\tFIRST\tLAST\tCOUNT")

	for _, r := range rows {
		def := "-"
		if r.HasDefault {
			def = fmt.Sprintf("%q", r.DefaultValue)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%d\n",
			r.Key,
			r.Value,
			r.Source,
			def,
			r.FirstRequested.Format("2006-01-02 15:04:05.000"),
			r.LastRequested.Format("2006-01-02 15:04:05.000"),
			r.Count,
		)
	}

	_ = w.Flush()
	return builder.String()
}

type settingRow struct {
	Key    string
	Value  string
	Source string
}

func (c *Configuration) settingsSnapshot() []settingRow {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keysMap := make(map[string]struct{})
	for item := range c.confs {
		keysMap[strings.Split(item, ".")[0]] = struct{}{}
	}

	keysArr := make([]string, 0, len(keysMap))
	for k := range keysMap {
		keysArr = append(keysArr, k)
	}
	sort.Strings(keysArr)

	rows := make([]settingRow, 0, len(keysArr))
	for _, k := range keysArr {
		v, _, source := c.getInternal(k)
		v = maskSecrets(v)
		rows = append(rows, settingRow{Key: k, Value: v, Source: source})
	}

	return rows
}

func (c *Configuration) requestCountByKey() map[string]int64 {
	c.rmu.RLock()
	defer c.rmu.RUnlock()

	m := make(map[string]int64)
	for _, rec := range c.requests {
		m[rec.Key] += rec.Count
	}

	return m
}

func (c *Configuration) Stats() string {
	var builder strings.Builder
	builder.WriteString("\nCMDLINE\n")
	builder.WriteString("-------\n")

	for i, arg := range os.Args {
		builder.WriteString(fmt.Sprintf("%2d: %s\n", i, arg))
	}

	builder.WriteString("\nSETTINGS_ENV\n")
	builder.WriteString("------------\n")
	builder.WriteString("Context:     ")

	if c.context != "dev" {
		builder.WriteString(c.context)
	} else {
		builder.WriteString("Not set (dev)")
	}

	builder.WriteString("\nApplication: ")
	if c.app != "" {
		builder.WriteString(c.app)
	} else {
		builder.WriteString("Not set")
	}

	builder.WriteString("\n\nSETTINGS\n--------\n")

	for _, row := range c.settingsSnapshot() {
		context := strings.Replace(row.Source, row.Key, "", 1)
		if context != "" {
			builder.WriteString(fmt.Sprintf("%s[%s]=%s\n", row.Key, context, row.Value))
		} else {
			builder.WriteString(fmt.Sprintf("%s=%s\n", row.Key, row.Value))
		}
	}

	return builder.String()
}

// Get context
func (c *Configuration) GetContext() string {
	return c.context
}

func (c *Configuration) AddListener(listener SettingsListener) {
	c.listenerMu.Lock()
	defer c.listenerMu.Unlock()

	c.listeners = append(c.listeners, listener)
}

func (c *Configuration) RemoveListener(listener SettingsListener) {
	c.listenerMu.Lock()
	defer c.listenerMu.Unlock()

	for i, l := range c.listeners {
		if l == listener {
			c.listeners = append(c.listeners[:i], c.listeners[i+1:]...)
			return
		}
	}
}

func HandleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	Config().printConfigHTML(w)
}

func (c *Configuration) printConfigHTML(p io.Writer) {
	settings := c.settingsSnapshot()
	counts := c.requestCountByKey()
	requested := c.requestedSnapshot()

	fmt.Fprintf(p, `<html>
<head>
<title>GoCore Configuration</title>
<script src="https://cdnjs.cloudflare.com/ajax/libs/jquery/1.4.3/jquery.min.js" integrity="sha512-xqRHwg8Pg0JQ+nne5mBy3SGrGDihpsr5UYuMgIcVj1SMfSKrRJNvu7tFitaK70xDpSsBBIVpTcTGXnmx7/Q2xw==" crossorigin="anonymous" referrerpolicy="no-referrer"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/jquery.tablesorter/2.31.3/js/jquery.tablesorter.min.js" integrity="sha512-qzgd5cYSZcosqpzpn7zF2ZId8f/8CHmFKZ8j7mU4OUXTNRd5g+ZHBPsgKEwoqxCtdQvExE5LprwwPAgoicguNg==" crossorigin="anonymous" referrerpolicy="no-referrer"></script>
<link rel='stylesheet' href='%scss/statistics.css' type='text/css' media='print, projection, screen' />
<script type='text/javascript'>
$(document).ready(function() {
	$('#settingsTable').tablesorter({ sortList: [[3,1]], widgets: ['zebra', 'saveSort'], headers: { 0: {sorter:'text'}, 1: {sorter:'text'}, 2: {sorter:'text'}, 3: {sorter:'number'} }, widgetOptions: { saveSort: true } });
	$('#requestedTable').tablesorter({ sortList: [[0,0]], widgets: ['zebra', 'saveSort'], headers: { 0: {sorter:'text'}, 1: {sorter:'text'}, 2: {sorter:'text'}, 3: {sorter:'text'}, 4: {sorter:'usLongDate'}, 5: {sorter:'usLongDate'}, 6: {sorter:'number'} }, widgetOptions: { saveSort: true } });
});
</script>
</head>
<body>
<h1>GoCore Configuration</h1>
<h2>Settings</h2>
<table id='settingsTable' class='tablesorter' border='0' cellpadding='0' cellspacing='1'>
<thead><tr><th>Key</th><th>Value</th><th>Source</th><th>Requests</th></tr></thead>
<tbody>
`, statPrefix)

	for _, s := range settings {
		fmt.Fprintf(p, "<tr><td>%s</td><td>%s</td><td>%s</td><td align='right'>%d</td></tr>\r\n",
			html.EscapeString(s.Key),
			html.EscapeString(s.Value),
			html.EscapeString(s.Source),
			counts[s.Key],
		)
	}

	fmt.Fprintf(p, `</tbody>
</table>
<h2>Requested</h2>
<table id='requestedTable' class='tablesorter' border='0' cellpadding='0' cellspacing='1'>
<thead><tr><th>Key</th><th>Value</th><th>Source</th><th>Default</th><th>First</th><th>Last</th><th>Count</th></tr></thead>
<tbody>
`)

	for _, rq := range requested {
		def := "-"
		if rq.HasDefault {
			def = rq.DefaultValue
		}

		fmt.Fprintf(p, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td align='right'>%d</td></tr>\r\n",
			html.EscapeString(rq.Key),
			html.EscapeString(rq.Value),
			html.EscapeString(rq.Source),
			html.EscapeString(def),
			rq.FirstRequested.Format("2006-01-02 15:04:05.000"),
			rq.LastRequested.Format("2006-01-02 15:04:05.000"),
			rq.Count,
		)
	}

	fmt.Fprintf(p, "</tbody>\r\n</table>\r\n</body></html>\r\n")
}
