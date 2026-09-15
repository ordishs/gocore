package gocore

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// xssPayloads are the delimiters an attacker needs to escape the inline
// JavaScript string, the HTML attribute, and the element itself.
var xssPayloads = []struct {
	name    string
	payload string
}{
	{"double quote", `");alert(1)//`},
	{"single quote", `');alert(1)//`},
	{"attribute break", `x' onmouseover='alert(1)`},
	{"element break", `x"><img src=x onerror=alert(1)>`},
	{"angle brackets", `<script>alert(1)</script>`},
	{"backslash", `x\");alert(1)//`},
	{"encoded delimiter", `x%22%3E%3Cimg%20src=x%20onerror=alert(1)%3E`},
}

func TestHandleStatsDoesNotEmitRawKeyInResetControl(t *testing.T) {
	for _, tc := range xssPayloads {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/stats?key="+urlQueryEscapeForTest(tc.payload), nil)
			rec := httptest.NewRecorder()

			HandleStats(rec, req)

			body := rec.Body.String()
			if strings.Contains(body, tc.payload) {
				t.Fatalf("response contains the raw payload %q", tc.payload)
			}
		})
	}
}

func TestHandleStatsEmitsNoInlineEventHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	rec := httptest.NewRecorder()

	HandleStats(rec, req)

	body := strings.ToLower(rec.Body.String())
	if strings.Contains(body, "onclick=") {
		t.Fatal("response still renders an inline onClick handler")
	}
}

// A statistic key is chosen by the consumer, not by the request, so validation
// of the query parameter cannot protect this sink. The renderer must escape it.
func TestHandleStatsEscapesHostileStatKeyInChildLink(t *testing.T) {
	hostile := `evil'><img src=x onerror=alert(1)>`

	NewStat(hostile).NewStat("child")

	rec := httptest.NewRecorder()
	HandleStats(rec, httptest.NewRequest(http.MethodGet, "/stats", nil))

	body := rec.Body.String()
	if strings.Contains(body, "<img src=x") {
		t.Fatalf("child link emitted the hostile stat key unescaped")
	}

	if !strings.Contains(body, "&lt;img") && !strings.Contains(body, "%3Cimg") {
		t.Fatal("hostile stat key does not appear in any escaped form; the sink may not have rendered")
	}
}

func TestHandleStatsEscapesHostileStatKeyInLeafCell(t *testing.T) {
	hostile := `leafevil'><img src=y onerror=alert(1)>`

	NewStat(hostile)

	rec := httptest.NewRecorder()
	HandleStats(rec, httptest.NewRequest(http.MethodGet, "/stats", nil))

	if strings.Contains(rec.Body.String(), "<img src=y") {
		t.Fatal("leaf cell emitted the hostile stat key unescaped")
	}
}

// A key that is legal under the grammar can still carry characters that are
// significant in a URL. They must be percent-encoded in the child link.
func TestHandleStatsEncodesUrlSignificantCharactersInChildLink(t *testing.T) {
	parent := "url sig#key"

	NewStat(parent).NewStat("child").NewStat("grandchild")

	rec := httptest.NewRecorder()
	HandleStats(rec, httptest.NewRequest(http.MethodGet, "/stats", nil))

	body := rec.Body.String()
	if !strings.Contains(body, "url+sig%23key") {
		t.Fatal("child link did not percent-encode the key into the query string")
	}
}

func TestHandleStatsRejectsInvalidKeyGrammarBeforeRendering(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/stats?key="+urlQueryEscapeForTest(`<script>alert(1)</script>`), nil)
	rec := httptest.NewRecorder()

	HandleStats(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if strings.Contains(rec.Body.String(), "<html") {
		t.Fatal("handler rendered HTML for an invalid key instead of rejecting it")
	}
}

func TestHandleStatsAcceptsOrdinaryKeys(t *testing.T) {
	NewStat("ordinary key-1.2").NewStat("child")

	req := httptest.NewRequest(http.MethodGet, "/stats?key="+urlQueryEscapeForTest("ordinary key-1.2"), nil)
	rec := httptest.NewRecorder()

	HandleStats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d for a legitimate key, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleStatsSetsContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	rec := httptest.NewRecorder()

	HandleStats(rec, req)

	got := rec.Header().Get("Content-Type")
	if got != "text/html; charset=utf-8" {
		t.Fatalf("got Content-Type %q, want %q", got, "text/html; charset=utf-8")
	}
}

func TestHandleStatsSetsContentSecurityPolicyWithoutUnsafeInlineScript(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	rec := httptest.NewRecorder()

	HandleStats(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("handler emitted no Content-Security-Policy header")
	}

	scriptSrc := directiveForTest(csp, "script-src")
	if scriptSrc == "" {
		t.Fatalf("CSP %q has no script-src directive", csp)
	}

	if strings.Contains(scriptSrc, "'unsafe-inline'") {
		t.Fatalf("script-src allows 'unsafe-inline': %q", scriptSrc)
	}

	if !strings.Contains(scriptSrc, "'nonce-") {
		t.Fatalf("script-src carries no nonce: %q", scriptSrc)
	}
}

func TestHandleStatsNonceInHeaderMatchesInlineScripts(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	rec := httptest.NewRecorder()

	HandleStats(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")

	nonce := nonceFromCSPForTest(csp)
	if nonce == "" {
		t.Fatalf("could not read a nonce from CSP %q", csp)
	}

	body := rec.Body.String()

	for _, fragment := range inlineScriptOpenTagsForTest(body) {
		if !strings.Contains(fragment, "nonce='"+nonce+"'") && !strings.Contains(fragment, `nonce="`+nonce+`"`) {
			t.Fatalf("inline script tag carries no matching nonce: %s", fragment)
		}
	}
}

func TestHandleStatsNonceDiffersPerResponse(t *testing.T) {
	first := httptest.NewRecorder()
	HandleStats(first, httptest.NewRequest(http.MethodGet, "/stats", nil))

	second := httptest.NewRecorder()
	HandleStats(second, httptest.NewRequest(http.MethodGet, "/stats", nil))

	a := nonceFromCSPForTest(first.Header().Get("Content-Security-Policy"))
	b := nonceFromCSPForTest(second.Header().Get("Content-Security-Policy"))

	if a == "" || b == "" {
		t.Fatal("a response carried no nonce")
	}

	if a == b {
		t.Fatalf("nonce was reused across responses: %q", a)
	}
}

func TestHandleStatsResetTargetHonoursStatPrefix(t *testing.T) {
	original := statPrefix

	defer func() { statPrefix = original }()

	for _, prefix := range []string{"/", "/stats/", "/a/b/"} {
		statPrefix = prefix

		rec := httptest.NewRecorder()
		HandleStats(rec, httptest.NewRequest(http.MethodGet, prefix+"stats", nil))

		body := rec.Body.String()
		if !strings.Contains(body, prefix+"css/statistics.css") {
			t.Fatalf("prefix %q not honoured in rendered page", prefix)
		}

		if strings.Contains(strings.ToLower(body), "onclick=") {
			t.Fatalf("prefix %q still renders an inline onClick handler", prefix)
		}
	}
}

func urlQueryEscapeForTest(s string) string {
	return url.QueryEscape(s)
}

func directiveForTest(csp, name string) string {
	for _, d := range strings.Split(csp, ";") {
		d = strings.TrimSpace(d)
		if strings.HasPrefix(d, name+" ") {
			return d
		}
	}

	return ""
}

func nonceFromCSPForTest(csp string) string {
	i := strings.Index(csp, "'nonce-")
	if i < 0 {
		return ""
	}

	rest := csp[i+len("'nonce-"):]

	j := strings.Index(rest, "'")
	if j < 0 {
		return ""
	}

	return rest[:j]
}

func inlineScriptOpenTagsForTest(body string) []string {
	var tags []string

	rest := body

	for {
		i := strings.Index(rest, "<script")
		if i < 0 {
			return tags
		}

		rest = rest[i:]

		j := strings.Index(rest, ">")
		if j < 0 {
			return tags
		}

		tag := rest[:j+1]
		rest = rest[j+1:]

		// Only inline scripts need a nonce; sourced scripts are covered by the
		// host allow-list in the CSP.
		if !strings.Contains(tag, "src=") {
			tags = append(tags, tag)
		}
	}
}

// resetRequest builds the form POST that the stats page submits.
func resetRequest(key string, headers map[string]string) *http.Request {
	form := url.Values{}
	if key != "" {
		form.Set("key", key)
	}

	req := httptest.NewRequest(http.MethodPost, "/reset", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return req
}

func TestResetStatsRejectsInvalidKeyGrammar(t *testing.T) {
	rec := httptest.NewRecorder()

	ResetStats(rec, resetRequest(`<script>alert(1)</script>`, nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestResetStatsHandlesUnknownKeyWithoutPanic(t *testing.T) {
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ResetStats panicked on an unknown key: %v", r)
		}
	}()

	ResetStats(rec, resetRequest("nosuchkeyatall", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("got status %d for an unknown key, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestResetStatsRejectsGet(t *testing.T) {
	rec := httptest.NewRecorder()

	ResetStats(rec, httptest.NewRequest(http.MethodGet, "/reset?key=", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("got status %d for GET, want %d", rec.Code, http.StatusMethodNotAllowed)
	}

	if allow := rec.Header().Get("Allow"); allow != http.MethodPost {
		t.Fatalf("got Allow %q, want %q", allow, http.MethodPost)
	}
}

// waitForSamples blocks until the statistic has accumulated a sample. AddTime
// enqueues the sample and a background goroutine accumulates it.
func waitForSamples(t *testing.T, key string) *Stat {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)

	for time.Now().Before(deadline) {
		stat := RootStat.getChild(key)
		if stat != nil {
			stat.mu.RLock()
			count := stat.count
			stat.mu.RUnlock()

			if count > 0 {
				return stat
			}
		}

		time.Sleep(time.Millisecond)
	}

	t.Fatalf("statistic %q recorded no samples", key)

	return nil
}

func countOf(t *testing.T, s *Stat) int64 {
	t.Helper()

	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.count
}

// The key must come from the POST body. If the handler reads it from anywhere
// else it resets the whole tree, and the bystander statistic is cleared too.
func TestResetStatsClearsOnlyTheNamedStatistic(t *testing.T) {
	NewStat("resettarget").AddTime(time.Now().Add(-time.Second))
	NewStat("resetbystander").AddTime(time.Now().Add(-time.Second))

	target := waitForSamples(t, "resettarget")
	bystander := waitForSamples(t, "resetbystander")

	rec := httptest.NewRecorder()
	ResetStats(rec, resetRequest("resettarget", nil))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusSeeOther)
	}

	if got := countOf(t, target); got != 0 {
		t.Fatalf("target count is %d after the reset, want 0", got)
	}

	if got := countOf(t, bystander); got == 0 {
		t.Fatal("bystander statistic was reset; the handler ignored the posted key")
	}
}

func TestResetStatsAcceptsSameOriginAndDirectRequests(t *testing.T) {
	NewStat("csrfallowed")

	allowed := []map[string]string{
		nil,
		{"Sec-Fetch-Site": "same-origin"},
		{"Sec-Fetch-Site": "none"},
		{"Origin": "http://example.com"},
		{"Origin": "https://example.com", "Sec-Fetch-Site": "same-origin"},
	}

	for i, headers := range allowed {
		rec := httptest.NewRecorder()

		req := resetRequest("csrfallowed", headers)
		req.Host = "example.com"

		ResetStats(rec, req)

		if rec.Code != http.StatusSeeOther {
			t.Fatalf("case %d: got status %d, want %d (headers %v)", i, rec.Code, http.StatusSeeOther, headers)
		}
	}
}

func TestResetStatsRejectsCrossOriginRequests(t *testing.T) {
	NewStat("csrfblocked")

	blocked := []map[string]string{
		{"Sec-Fetch-Site": "cross-site"},
		{"Sec-Fetch-Site": "same-site"},
		{"Origin": "http://attacker.example"},
		{"Origin": "http://example.com.attacker.example"},
	}

	for i, headers := range blocked {
		rec := httptest.NewRecorder()

		req := resetRequest("csrfblocked", headers)
		req.Host = "example.com"

		ResetStats(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("case %d: got status %d, want %d (headers %v)", i, rec.Code, http.StatusForbidden, headers)
		}
	}
}

func TestStatsPageRendersPostResetFormWithoutScript(t *testing.T) {
	rec := httptest.NewRecorder()
	HandleStats(rec, httptest.NewRequest(http.MethodGet, "/stats?key=", nil))

	body := rec.Body.String()

	if !strings.Contains(strings.ToLower(body), "method='post'") {
		t.Fatal("reset form is not a POST form")
	}

	if !strings.Contains(body, "<input type='hidden' name='key'") {
		t.Fatal("reset form carries no hidden key field")
	}

	if strings.Contains(body, "resetStatsButton") {
		t.Fatal("reset still depends on a scripted button")
	}
}
