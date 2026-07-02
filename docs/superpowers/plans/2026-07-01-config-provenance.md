# Config provenance & `/config` endpoint Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enrich gocore's requested-settings tracking with provenance (source, default, first/last timestamps, count), mask secrets in that output, and expose it over the existing socket plus a new sortable `/config` HTTP endpoint.

**Architecture:** `getInternal()` remains the single pure resolver returning `(value, ok, source)`. A new `record()` upserts a `requestRecord` per distinct `(key, hasDefault, default)` call-site, masking any `*EHE*` token via a shared regex. Every getter (string + typed) records its own default. Two snapshot functions (`requestedSnapshot`, `settingsSnapshot`) feed both a text formatter (socket `config requested`, `config show`) and an HTML renderer (`/config`).

**Tech Stack:** Go (stdlib `regexp`, `sort`, `text/tabwriter`, `html`, `net/http`, `net/http/httptest`), testify.

## Global Constraints

- Language: Go. No new third-party dependencies (stdlib + existing testify only).
- `Stats()` text output (startup printout + socket `config show`) must remain byte-for-byte identical.
- Env-override mechanism is unchanged: overrides are looked up by the bare key name via `os.LookupEnv`.
- EHE mask string is exactly `********************` (20 asterisks), matching current `Stats()`.
- Tests share the package-level `Config()` singleton; its `requests` map and process env accumulate across tests. New tests MUST use unique key/env names and assert only on settings not mutated elsewhere (safe: `name`, `tel`, `secret`, `magicNumber`).
- After editing any Go file, run: `gci write --skip-generated -s standard -s default <file>`.
- Commit messages: conventional-commit style, imperative, no attribution footers. Use `git commit --no-verify`.

---

### Task 1: Request tracking core — struct, `record()`, `Get`, snapshot, `Requested()`

Changing `requests` from `map[string]string` to `map[string]*requestRecord` breaks the current `Requested()` (which indexes it as a string), so the map change, recorder, string `Get`, snapshot, and `Requested()` rewrite ship together as one compiling, testable unit.

**Files:**
- Modify: `config.go` (struct/vars ~27-49, `requests` init ~239 & ~429, `Get` ~548-557, `Requested` ~955-984)
- Test: `config_test.go`

**Interfaces:**
- Consumes: existing `getInternal(key string, defaultValue ...string) (string, bool, string)` (unchanged).
- Produces:
  - `type requestRecord struct { Key, DefaultValue string; HasDefault bool; Value, Source string; FirstRequested, LastRequested time.Time; Count int64 }`
  - package vars `var reEHE = regexp.MustCompile(...)` and `const eheMask = "********************"`
  - `func (c *Configuration) record(key string, hasDefault bool, defaultStr, value, source string)`
  - `func (c *Configuration) requestedSnapshot() []requestRecord`
  - `func (c *Configuration) Requested() string` (rewritten)

- [ ] **Step 1: Write the failing tests**

Append to `config_test.go`:

```go
func TestRequestRecordsDistinctByDefault(t *testing.T) {
	Config().Get("distinct_key", "ABC")
	Config().Get("distinct_key", "DEF")
	Config().Get("distinct_key")

	var count int
	for _, r := range Config().requestedSnapshot() {
		if r.Key == "distinct_key" {
			count++
		}
	}
	assert.Equal(t, 3, count)
}

func TestRequestSource(t *testing.T) {
	Config().Get("src_missing_key")

	os.Setenv("src_env_key", "hello")
	Config().Get("src_env_key")

	Config().Get("tel")

	src := func(key string) string {
		for _, r := range Config().requestedSnapshot() {
			if r.Key == key {
				return r.Source
			}
		}
		return ""
	}

	assert.Equal(t, "DEFAULT", src("src_missing_key"))
	assert.Equal(t, "ENV", src("src_env_key"))
	assert.Equal(t, "tel", src("tel"))
}

func TestRequestCountAndTimes(t *testing.T) {
	find := func() requestRecord {
		for _, r := range Config().requestedSnapshot() {
			if r.Key == "times_key" && r.HasDefault && r.DefaultValue == "x" {
				return r
			}
		}
		return requestRecord{}
	}

	Config().Get("times_key", "x")
	r1 := find()
	require.Equal(t, int64(1), r1.Count)

	time.Sleep(2 * time.Millisecond)

	Config().Get("times_key", "x")
	r2 := find()
	assert.Equal(t, int64(2), r2.Count)
	assert.Equal(t, r1.FirstRequested, r2.FirstRequested)
	assert.True(t, r2.LastRequested.After(r1.FirstRequested))
}

func TestRequestedMasksEHE(t *testing.T) {
	v, ok := Config().Get("secret")
	require.True(t, ok)
	assert.Equal(t, "secret", v)

	for _, r := range Config().requestedSnapshot() {
		if r.Key == "secret" {
			assert.Equal(t, eheMask, r.Value)
			return
		}
	}
	t.Fatal("secret was not recorded")
}

func TestRequestedTextHeader(t *testing.T) {
	Config().Get("name")
	out := Config().Requested()
	assert.Contains(t, out, "KEY")
	assert.Contains(t, out, "SOURCE")
	assert.Contains(t, out, "COUNT")
	assert.Contains(t, out, "name")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./ -run 'TestRequest' -v`
Expected: FAIL — compile errors (`requestRecord`, `requestedSnapshot`, `eheMask` undefined).

- [ ] **Step 3: Add the struct, shared regex/mask, and `record()`**

In `config.go`, add package-level declarations near the other `var (...)` block (after line ~49):

```go
var reEHE = regexp.MustCompile(`(\*EHE\*[a-zA-Z0-9]+)`)

const eheMask = "********************"

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
```

Add the `record` method (place it just above `Get`):

```go
func (c *Configuration) record(key string, hasDefault bool, defaultStr, value, source string) {
	masked := reEHE.ReplaceAllString(value, eheMask)

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
		DefaultValue:   defaultStr,
		HasDefault:     hasDefault,
		Value:          masked,
		Source:         source,
		FirstRequested: now,
		LastRequested:  now,
		Count:          1,
	}
}
```

- [ ] **Step 4: Change the map type and initializers**

In the `Configuration` struct (line ~31) change:

```go
	requests   map[string]string
```
to:
```go
	requests   map[string]*requestRecord
```

Update the two initializers:
- line ~239: `c.requests = make(map[string]*requestRecord)`
- line ~429: `ac.requests = make(map[string]*requestRecord)`

- [ ] **Step 5: Refactor `Get` to record provenance**

Replace `Get` (lines ~548-557):

```go
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
```

- [ ] **Step 6: Add `requestedSnapshot` and rewrite `Requested`**

Replace `Requested` (lines ~955-984) with:

```go
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
```

Add `"text/tabwriter"` to the import block.

- [ ] **Step 7: Format and run tests**

Run: `gci write --skip-generated -s standard -s default config.go config_test.go && go test ./ -run 'TestRequest' -v`
Expected: PASS (all five `TestRequest*` tests).

- [ ] **Step 8: Run the full package suite (guard against regressions)**

Run: `go test ./ -v`
Expected: PASS. (Existing `Get`/typed-getter tests still pass; typed getters still route through `c.Get` at this point.)

- [ ] **Step 9: Commit**

```bash
git add config.go config_test.go
git commit --no-verify -m "feat(config): track request provenance and mask secrets in requested output"
```

---

### Task 2: Typed getters record their own default + source

**Files:**
- Modify: `config.go` (`getNumber` ~641-711, `GetMulti` ~619-633, `GetBool` ~869-884, `GetDuration` ~886-900, `GetURL` ~902-933)
- Test: `config_test.go`

**Interfaces:**
- Consumes: `getInternal`, `record`, `reEHE` (Task 1).
- Produces: unchanged public signatures; each getter now calls `record()` with its own default.

- [ ] **Step 1: Write the failing tests**

Append to `config_test.go`:

```go
func TestTypedGetterRecordsDefault(t *testing.T) {
	Config().GetInt("typed_missing", 99)

	for _, r := range Config().requestedSnapshot() {
		if r.Key == "typed_missing" {
			assert.True(t, r.HasDefault)
			assert.Equal(t, "99", r.DefaultValue)
			assert.Equal(t, "99", r.Value)
			assert.Equal(t, "DEFAULT", r.Source)
			return
		}
	}
	t.Fatal("typed_missing was not recorded")
}

func TestBoolGetterRecordsDefault(t *testing.T) {
	Config().GetBool("bool_missing", true)

	for _, r := range Config().requestedSnapshot() {
		if r.Key == "bool_missing" {
			assert.True(t, r.HasDefault)
			assert.Equal(t, "true", r.DefaultValue)
			assert.Equal(t, "DEFAULT", r.Source)
			return
		}
	}
	t.Fatal("bool_missing was not recorded")
}

func TestURLGetterRecordsSource(t *testing.T) {
	_, err, _ := Config().GetURL("url1")
	require.NoError(t, err)

	for _, r := range Config().requestedSnapshot() {
		if r.Key == "url1" {
			assert.Equal(t, "url1", r.Source)
			return
		}
	}
	t.Fatal("url1 was not recorded")
}

func TestDurationGetterRecordsFoundSource(t *testing.T) {
	_, err, ok := Config().GetDuration("millis")
	require.NoError(t, err)
	require.True(t, ok)

	for _, r := range Config().requestedSnapshot() {
		if r.Key == "millis" {
			assert.Equal(t, "millis", r.Source)
			assert.Equal(t, "2s", r.Value)
			return
		}
	}
	t.Fatal("millis was not recorded")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./ -run 'TestTypedGetterRecordsDefault|TestBoolGetterRecordsDefault|TestURLGetterRecordsSource|TestDurationGetterRecordsFoundSource' -v`
Expected: FAIL — e.g. `typed_missing` recorded with `HasDefault=false` / empty default (typed getters currently call `c.Get(key)` with no default).

- [ ] **Step 3: Refactor `getNumber`**

Replace the head of `getNumber` (the `str, ok := c.Get(key)` block and default handling, lines ~642-650) so the whole function reads:

```go
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
```

- [ ] **Step 4: Refactor `GetMulti`**

```go
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
```

- [ ] **Step 5: Refactor `GetBool`**

```go
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
```

- [ ] **Step 6: Refactor `GetDuration`**

```go
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
```

- [ ] **Step 7: Refactor `GetURL`** (reuse `reEHE`; record before embedded-token decryption so tokens are masked)

```go
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
```

Note: the local `re := regexp.MustCompile(...)` previously inside `GetURL` is removed — `reEHE` replaces it.

- [ ] **Step 8: Format and run the targeted + full suites**

Run: `gci write --skip-generated -s standard -s default config.go config_test.go && go test ./ -v`
Expected: PASS. In particular the four new tests plus existing `TestGetUint*`, `TestGetDuration`, `TestURL*`, `TestGetMagicNumber`, `TestEncryptDecryptInt`.

- [ ] **Step 9: Commit**

```bash
git add config.go config_test.go
git commit --no-verify -m "feat(config): record typed getter defaults and sources"
```

---

### Task 3: Stop `replaceVariables` polluting the requested map

**Files:**
- Modify: `config.go` (`replaceVariables` ~528-546)
- Test: `config_test.go`

**Interfaces:**
- Consumes: `getInternal`.
- Produces: `replaceVariables` no longer calls the recording `Get`.

- [ ] **Step 1: Write the failing test**

Append to `config_test.go`:

```go
func TestReplaceVariablesNoPollution(t *testing.T) {
	Config().Set("polvar", "hello")
	Config().Set("uses_polvar", "prefix ${polvar}")

	val, ok := Config().Get("uses_polvar")
	require.True(t, ok)
	assert.Equal(t, "prefix hello", val)

	for _, r := range Config().requestedSnapshot() {
		if r.Key == "polvar" {
			t.Fatal("interpolation-only var 'polvar' must not be recorded as requested")
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ -run TestReplaceVariablesNoPollution -v`
Expected: FAIL — `polvar` is recorded because `replaceVariables` calls `c.Get`.

- [ ] **Step 3: Switch `replaceVariables` to the non-recording resolver**

Replace the inner lookup in `replaceVariables` (lines ~536-542):

```go
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
```

- [ ] **Step 4: Run tests**

Run: `go test ./ -run 'TestReplaceVariablesNoPollution|TestDynamicVariables|TestEnvWithVariables|TestGetEmbedded1' -v`
Expected: PASS (interpolation behavior preserved; no pollution).

- [ ] **Step 5: Format, full suite, commit**

```bash
gci write --skip-generated -s standard -s default config.go config_test.go
go test ./ -v
git add config.go config_test.go
git commit --no-verify -m "fix(config): exclude variable interpolation from requested tracking"
```
Expected: full suite PASS.

---

### Task 4: `settingsSnapshot`, `Stats()` refactor, `requestCountByKey`

**Files:**
- Modify: `config.go` (`Stats` ~987-1048; add `settingsSnapshot`, `requestCountByKey`, `settingRow`)
- Test: `config_test.go`

**Interfaces:**
- Consumes: `getInternal`, `reEHE`, `eheMask`, `requests`.
- Produces:
  - `type settingRow struct { Key, Value, Source string }`
  - `func (c *Configuration) settingsSnapshot() []settingRow`
  - `func (c *Configuration) requestCountByKey() map[string]int64`
  - `Stats()` rebuilt on top of `settingsSnapshot` (output unchanged).

- [ ] **Step 1: Write the failing tests**

Append to `config_test.go`:

```go
func TestStatsFormatUnchanged(t *testing.T) {
	s := Config().Stats()

	assert.Contains(t, s, "\nCMDLINE\n-------\n")
	assert.Contains(t, s, "\nSETTINGS\n--------\n")
	assert.Contains(t, s, "name=Simon\n")
	assert.Contains(t, s, "tel=20289202982\n")
	assert.Contains(t, s, "secret="+eheMask+"\n")
	assert.Contains(t, s, "magicNumber="+eheMask+"\n")
}

func TestRequestCountByKey(t *testing.T) {
	Config().Get("reqcount_key")
	Config().Get("reqcount_key")

	counts := Config().requestCountByKey()
	assert.GreaterOrEqual(t, counts["reqcount_key"], int64(2))
	_, present := counts["reqcount_never_requested_key"]
	assert.False(t, present)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./ -run 'TestStatsFormatUnchanged|TestRequestCountByKey' -v`
Expected: FAIL — `requestCountByKey` undefined. (`TestStatsFormatUnchanged` may pass already; it becomes the regression guard for Step 3.)

- [ ] **Step 3: Add `settingRow`, `settingsSnapshot`, `requestCountByKey`; rebuild `Stats`**

Add near the other config types:

```go
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
		v = reEHE.ReplaceAllString(v, eheMask)
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
```

Replace `Stats` (lines ~987-1048) with:

```go
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
```

This removes the local `re := regexp.MustCompile(...)` and inline key-collection from `Stats` (now in `settingsSnapshot`, using the shared `reEHE`). The header text, `[context]` derivation, masking, and sort order are preserved exactly.

- [ ] **Step 4: Format and run tests**

Run: `gci write --skip-generated -s standard -s default config.go config_test.go && go test ./ -run 'TestStatsFormatUnchanged|TestRequestCountByKey' -v`
Expected: PASS.

- [ ] **Step 5: Full suite + commit**

```bash
go test ./ -v
git add config.go config_test.go
git commit --no-verify -m "refactor(config): extract settings snapshot and add per-key request counts"
```
Expected: full suite PASS.

---

### Task 5: `/config` HTTP endpoint (sortable, two tables)

**Files:**
- Modify: `config.go` (add `HandleConfig`, `printConfigHTML`; add imports `html`)
- Modify: `Stat.go` (register route in `RegisterStatsHandlers` ~292-296; add nav link in `printStatisticsHTML` ~471-473)
- Test: `config_http_test.go` (new)

**Interfaces:**
- Consumes: `settingsSnapshot`, `requestCountByKey`, `requestedSnapshot` (Tasks 1 & 4), `statPrefix` (Stat.go).
- Produces:
  - `func HandleConfig(w http.ResponseWriter, r *http.Request)`
  - `func (c *Configuration) printConfigHTML(p io.Writer)`

- [ ] **Step 1: Write the failing test**

Create `config_http_test.go`:

```go
package gocore

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandleConfig(t *testing.T) {
	Config().Get("name")

	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	rec := httptest.NewRecorder()

	HandleConfig(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	assert.Contains(t, body, "id='settingsTable'")
	assert.Contains(t, body, "id='requestedTable'")
	assert.Contains(t, body, "GoCore Configuration")
	assert.Contains(t, body, "Requests")
	assert.Contains(t, body, "name")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ -run TestHandleConfig -v`
Expected: FAIL — `HandleConfig` undefined.

- [ ] **Step 3: Add `HandleConfig` and `printConfigHTML`**

Add to `config.go` (and add `"html"` to the import block):

```go
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
```

- [ ] **Step 4: Register the route**

In `Stat.go`, inside `RegisterStatsHandlers`' loop (lines ~292-296), add the config route:

```go
			m.HandleFunc(statPrefix+"stats", HandleStats)
			m.HandleFunc(statPrefix+"config", HandleConfig)
			m.HandleFunc(statPrefix+"reset", ResetStats)
			m.HandleFunc(statPrefix+"", HandleOther)
```

- [ ] **Step 5: Add a nav link on the stats page**

In `Stat.go` `printStatisticsHTML`, immediately after the `GoCore Statistics\r\n` `<h1>` block (after line ~473 `</h1>`), add:

```go
	fmt.Fprintf(p, "<div><a href='%sconfig'>Configuration</a></div>\r\n", statPrefix)
```

- [ ] **Step 6: Format and run the test**

Run: `gci write --skip-generated -s standard -s default config.go Stat.go config_http_test.go && go test ./ -run TestHandleConfig -v`
Expected: PASS.

- [ ] **Step 7: Full suite + commit**

```bash
go test ./ -v
git add config.go Stat.go config_http_test.go
git commit --no-verify -m "feat(config): serve /config endpoint with sortable settings and requested tables"
```
Expected: full suite PASS.

---

## Self-Review

**Spec coverage:**
- Enriched record (value/source/default/first/last/count) → Task 1 struct + `record`; timestamps + count asserted in `TestRequestCountAndTimes`.
- Distinct entry per `(key, default)` → Task 1 composite `mapKey`; `TestRequestRecordsDistinctByDefault`.
- Source = ENV / resolved key / DEFAULT → Task 1 `Get`, Task 2 typed getters; `TestRequestSource`, `TestURLGetterRecordsSource`, `TestDurationGetterRecordsFoundSource`.
- All getters capture own default → Task 2; `TestTypedGetterRecordsDefault`, `TestBoolGetterRecordsDefault`.
- EHE masking → Task 1 `record` via `reEHE`; `TestRequestedMasksEHE`.
- `replaceVariables` no pollution → Task 3; `TestReplaceVariablesNoPollution`.
- Text `Requested()` table → Task 1; `TestRequestedTextHeader` (socket `config requested` consumes it unchanged).
- `Stats()` byte-identical → Task 4 refactor; `TestStatsFormatUnchanged`.
- `/config` always-on, two sortable tables, Requests-count column → Task 5; `TestHandleConfig` + route registration + nav link.
- Limitation (source = winning key, not file) and rustcore parity: documented in spec; no task (out of scope by decision).

**Placeholder scan:** No TBD/TODO/"handle edge cases"/"similar to". Every code step shows complete code.

**Type consistency:** `requestRecord`, `settingRow`, `record`, `requestedSnapshot`, `settingsSnapshot`, `requestCountByKey`, `HandleConfig`, `printConfigHTML`, `reEHE`, `eheMask` used identically across tasks. `getInternal` signature `(string, bool, string)` unchanged throughout. `Requested()`/`Stats()` signatures unchanged.
