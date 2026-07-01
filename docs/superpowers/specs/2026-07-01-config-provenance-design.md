# Config provenance & `/config` endpoint — design

## Problem

gocore prints all settings at startup so logs answer "what settings is this
process actually using" rather than "what is in the settings file today". Env
vars take highest precedence, but the startup printout (`Stats()`) only reflects
env overrides for keys that are *also declared* in a `.conf` file. It iterates
the keys found in `settings.conf` / `settings_test.conf` / `settings_local.conf`
and layers env values on top.

The gap: an env var that overrides a setting **never declared in any `.conf`
file** — e.g. code calls `Config().Get("EXTRA", "false")` and the environment
sets `EXTRA=true` — is live but invisible in the startup dump. Dumping the whole
environment is not viable: there is no way to tell `EXTRA` apart from `TERM`,
`HOME`, `PATH`, etc.

A namespace-prefix convention for override env vars was considered and
rejected (unwanted change to the bare-key override syntax).

The reliable signal already inside the process: every key passed to
`Get()`/`GetInt()`/etc. is recorded in `c.requests`. The set of "our" settings =
declared keys ∪ keys the app has ever requested. In practice this is
near-complete at startup, because several gocore apps load all settings into a
Go struct early in startup (replacing later `Config().Get` calls), so the
requested set is populated up front.

## Goals

- Enrich the requested-settings tracking so it records, per request:
  - the value actually served
  - where the value came from (env, resolved context/app key, or default)
  - the default value the caller supplied
  - first-requested and last-requested timestamps
  - request count
- Treat distinct call-sites as distinct entries: `Get("EXTRA","ABC")` and
  `Get("EXTRA","DEF")` are separate rows, each with the value served.
- Surface this through the existing `config requested` Unix-socket command
  (enriched text table) and a new `/config` HTTP endpoint (sortable HTML), the
  latter mirroring the existing `/stats` endpoint.
- Never leak decrypted secrets into the diagnostic output.

## Non-goals

- No change to the env-override mechanism (still by bare key name).
- No change to the byte-for-byte text output of `Stats()` (startup printout and
  `config show`).
- Reporting *which `.conf` file* a value came from (see Limitations).
- Mirroring the change into rustcore (flagged; done separately).

## Decisions (locked with the user)

1. **Capture scope:** all getters, not just string `Get`. Typed getters record
   their own typed default + source.
2. **Secrets:** mask EHE-sourced values in the diagnostic output.
3. **Timing:** record first-requested, last-requested, and count.
4. **`/config` access:** always on (like `/stats`). The stats server listens on
   localhost only, so exposure requires local or port-forward access.
5. **`/config` content:** two views — a Settings table (all declared settings)
   plus a Requested table (provenance). The Settings table carries a Requests
   count column so "declared in files but never requested" settings are
   visible at a glance (count = 0).
6. Both HTML tables are sortable (tablesorter), like `/stats`.

## Design

### A. Enriched request tracking (`config.go`)

Replace `requests map[string]string` with:

```go
type requestRecord struct {
    Key            string
    DefaultValue   string    // caller's default, formatted (e.g. "ABC", "30", "30s")
    HasDefault     bool      // Get("X") vs Get("X","") are distinct entries
    Value          string    // final value served (masked if Encrypted)
    Source         string    // "ENV" | resolved context/app key suffix | "DEFAULT"
    Encrypted      bool      // value originated from an *EHE* token
    FirstRequested time.Time
    LastRequested  time.Time
    Count          int64
}
```

`requests` becomes `map[string]*requestRecord`, keyed by a composite of `Key`,
`HasDefault`, and `DefaultValue` so distinct call-sites (and distinct defaults)
get distinct rows. Repeated identical calls bump `Count` and update
`Value/Source/LastRequested`, leaving `FirstRequested` untouched.

### B. Split resolution from recording

Today `getInternal()` both resolves and is the only place source is known;
`Get()` is the only recorder. Refactor to:

- `getInternal(key)` → `(value, ok, source, encrypted)`: **pure** resolution.
  No default applied, no recording. `encrypted` is true when the pre-decrypt
  value contained `*EHE*`.
- Private `record(key string, hasDefault bool, defaultStr, finalValue, source string, encrypted bool)`:
  upserts the map. First-seen stamps `FirstRequested`; every call updates
  `Value/Source/LastRequested` and bumps `Count`. When `encrypted`, the stored
  `Value` is masked (`********`) so the map never holds plaintext secrets — this
  is stronger than `Stats()`, which only masks tokens that *failed* to decrypt.

### C. Every getter records its own default

`Get`, `GetMulti`, `GetBool`, `GetDuration`, `GetURL`, and the generic
`getNumber` (backing all `GetInt*/GetUint*/GetFloat*`) each:

1. resolve the raw value via `getInternal(key)`,
2. apply their own typed default when not found (`Source` is then `"DEFAULT"`),
3. call `record()` with the formatted default + final value + source.

This captures complete provenance, including the real typed default for typed
getters, without double-recording.

### D. `replaceVariables()` cleanup

`replaceVariables()` currently calls `c.Get()` for each `${var}`, polluting the
requested map with interpolation lookups. It switches to `getInternal()` (no
recording), so only genuine app requests are tracked.

### E. Structured snapshots + text formatters (`config.go`)

Two snapshot producers give a single source of truth for the text and HTML
renderers:

- `requestedSnapshot() []requestRecord` — sorted copy under lock. Feeds:
  - `Requested() string` — enriched plain-text aligned table for the
    `config requested` socket command.
- `settingsSnapshot() []settingRow` where
  `settingRow{ Key, Value, Source string; Encrypted bool }` — a refactor of the
  existing `Stats()` internals. Feeds:
  - `Stats() string` — **output byte-for-byte identical** to today (startup
    printout + `config show`). Only the internals are refactored to consume the
    snapshot.

### F. `/config` HTTP endpoint

Registration mirrors `/stats`. Inside `RegisterStatsHandlers`' `registerOnce`
block (`Stat.go`), add:

```go
m.HandleFunc(statPrefix+"config", HandleConfig)
```

`ServeMux` longest-match places `statPrefix+"config"` ahead of the catch-all
`statPrefix+""`, exactly as `stats` sits today. Always registered.

`HandleConfig` (rendering in `config.go`) writes an HTML page reusing the
existing CDN `jquery.tablesorter` scripts and `statistics.css` (no new assets),
with two sortable tables against the default `Config()`:

1. **Settings** (`#settingsTable`), from `settingsSnapshot()` joined with
   requested counts:
   - Columns: KEY (text), VALUE (text), SOURCE (text), REQUESTS (numeric).
   - `REQUESTS` = sum of request counts across all requested records for that
     base key. `0` = declared but never requested (dead setting), sortable to
     the top.
2. **Requested** (`#requestedTable`), from `requestedSnapshot()`:
   - Columns: KEY (text), VALUE (text), SOURCE (text), DEFAULT (text),
     FIRST (date), LAST (date), COUNT (numeric).

Timestamps are rendered in the `2006-01-02 15:04:05.000` format `/stats` uses so
the `usLongDate` sorter applies. Zebra striping + `saveSort` widgets carried
over. A link from the stats index page → `/config` is added.

## Output examples

`config requested` (socket, text):

```
KEY         VALUE   SOURCE        DEFAULT  FIRST                    LAST                     COUNT
EXTRA       XYZ     .live.myapp   "ABC"    2026-07-01 10:00:02.001  2026-07-01 10:03:11.552  4
EXTRA       XYZ     .live.myapp   "DEF"    2026-07-01 10:00:02.010  2026-07-01 10:00:02.010  1
dbPassword  ******  ENV           -        2026-07-01 10:00:01.000  2026-07-01 10:00:01.000  1
timeout     30s     DEFAULT       "30s"    2026-07-01 10:00:03.100  2026-07-01 10:02:00.900  2
```

`/config` Settings table (HTML, illustrative):

```
KEY          VALUE   SOURCE        REQUESTS
listenAddr   :8080   .live         3
deadSetting  foo     (base)        0
dbPassword   ******  ENV           1
```

## Limitations

- `Source` resolves to the *winning context/app key* (e.g. `.live.myapp`), not
  *which `.conf` file* provided it. gocore merges all files into a single
  `confs` map, discarding file origin. Reporting the file would require extra
  origin tracking in `processFile`.
- Typed getters where the caller passes no default and the key is absent record
  `Source="DEFAULT"` with the getter's zero value.

## rustcore parity

`~/dev/rust/rustcore` mirrors gocore's config semantics. This changes the
requested-tracking model and adds `/config`; rustcore drifts until mirrored.
Tracked separately, not in this change.

## Testing

- Unit tests (`config_test.go`):
  - distinct records for `Get("K","A")` vs `Get("K","B")` vs `Get("K")`.
  - `Source` correctness for env override, context/app resolution, and default.
  - `Count`, `FirstRequested` stability, `LastRequested` advance across repeated
    calls.
  - EHE value masked in the record and in `Requested()` output.
  - typed getters (`GetInt`, `GetBool`, `GetDuration`, `GetURL`, `GetMulti`)
    record their default + correct source.
  - `replaceVariables()` no longer creates spurious requested entries.
  - `Stats()` text output unchanged (golden comparison).
- HTTP: `HandleConfig` returns 200 with both tables; REQUESTS=0 row present for
  a declared-but-unrequested setting.

## Files touched

- `config.go` — data model, resolution split, getter refactor,
  `replaceVariables`, snapshots, `Requested()`, `Stats()` internals,
  `HandleConfig` + HTML rendering.
- `Stat.go` — one registration line in `RegisterStatsHandlers`; stats index link.
- `config_test.go` — tests above.
