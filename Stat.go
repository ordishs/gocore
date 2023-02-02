package gocore

import (
	"embed"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ordishs/gocore/utils"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const REPORTED_TIME_THRESHOLD_MINUTES = 5
const REPORTED_TIME_THRESHOLD = REPORTED_TIME_THRESHOLD_MINUTES * time.Minute

var (
	//go:embed all:embed/*

	res embed.FS

	initTime = time.Now().UTC()
	RootStat = &Stat{
		key:                "root",
		children:           make(map[string]*Stat),
		ignoreChildUpdates: true,
	}
)

// Stat comment
type Stat struct {
	mu                 sync.RWMutex
	key                string
	parent             *Stat
	children           map[string]*Stat
	ignoreChildUpdates bool
	hideTotal          bool
	firstDuration      time.Duration
	lastDuration       time.Duration
	minDuration        time.Duration
	maxDuration        time.Duration
	totalDuration      time.Duration
	count              int64
	firstTime          time.Time
	lastTime           time.Time
}

func NewStat(key string, options ...bool) *Stat {
	return RootStat.NewStat(key, options...)
}

func (s *Stat) NewStat(key string, options ...bool) *Stat {
	s.mu.Lock()
	defer s.mu.Unlock()

	stat, ok := s.children[key]
	if !ok {
		stat = &Stat{
			key:      key,
			parent:   s,
			children: make(map[string]*Stat),
		}

		if len(options) > 0 {
			stat.ignoreChildUpdates = options[0]
		}

		s.children[key] = stat
	}

	return stat
}

func (s *Stat) HideTotal(b bool) {
	s.hideTotal = b
}

func (s *Stat) getChild(key string) *Stat {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.children[key]
}

func (s *Stat) processTime(now time.Time, duration time.Duration) {
	if duration > REPORTED_TIME_THRESHOLD {
		log.Printf("Stat: time for %s is greater than %d minutes", s.key, REPORTED_TIME_THRESHOLD_MINUTES)
		return
	}

	s.mu.Lock()

	s.lastTime = now
	s.lastDuration = duration

	if s.count == 0 {
		s.firstTime = now
		s.firstDuration = duration
		s.minDuration = duration
		s.maxDuration = duration
	} else {
		if duration < s.minDuration {
			s.minDuration = duration
		}
		if duration > s.maxDuration {
			s.maxDuration = duration
		}
	}
	s.totalDuration += duration
	s.count++

	s.mu.Unlock()

	if s.parent != nil && !s.parent.ignoreChildUpdates {
		s.parent.processTime(now, duration)
	}
}

// AddTime comment
func (s *Stat) AddTime(startNanos int64) int64 {
	now := time.Now().UTC()

	endNanos := now.UnixNano()

	if endNanos < startNanos {
		log.Printf("%s: EndNanos is less than StartNanos", s.key)
		return endNanos
	}

	diff := endNanos - startNanos

	s.processTime(now, time.Duration(diff))

	return endNanos
}

func (s *Stat) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.firstDuration = 0
	s.lastDuration = 0
	s.minDuration = 0
	s.maxDuration = 0
	s.totalDuration = 0
	s.count = 0
	s.firstTime = time.Time{}
	s.lastTime = time.Time{}

	for _, stat := range s.children {
		stat.reset()
	}
}

func CurrentNanos() int64 {
	return time.Now().UnixNano()
}

func (s *Stat) average() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.count == 0 {
		return 0
	}

	return time.Duration(s.totalDuration.Nanoseconds() / s.count)
}

func (s *Stat) String() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return fmt.Sprintf("%s (%t): %s (%d)", s.key, s.ignoreChildUpdates, utils.HumanTime(s.totalDuration), s.count)
}

func StartStatsServer(addr string) {
	logger := Log("stats")

	http.HandleFunc("/stats", handleStats)
	http.HandleFunc("/reset", resetStats)
	http.HandleFunc("/", handleOther)

	logger.Infof("Starting StatsServer on http://%s/stats", addr)
	var err = http.ListenAndServe(addr, nil)

	if err != nil {
		logger.Panicf("Server failed starting. Error: %s", err)
	}
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	keysParam := r.URL.Query().Get("key")
	RootStat.mu.RLock()
	defer RootStat.mu.RUnlock()

	RootStat.printStatisticsHTML(w, RootStat, keysParam)
}

func resetStats(w http.ResponseWriter, r *http.Request) {
	keysParam := r.URL.Query().Get("key")
	item := RootStat

	if keysParam != "" {
		for _, key := range strings.Split(keysParam, ",") {
			item = item.getChild(key)
		}
	}

	item.reset()
	http.Redirect(w, r, "/stats", http.StatusSeeOther)
}

func handleOther(w http.ResponseWriter, r *http.Request) {
	var resource string

	path := r.URL.Path

	if path == "/" {
		resource = "embed/index.html"
	} else {
		resource = fmt.Sprintf("embed%s", path)
	}

	b, err := res.ReadFile(resource)
	if err != nil {
		// Just in case we're missing the /index.html, add it and try again...
		resource += "/index.html"
		b, err = res.ReadFile(resource)
		if err != nil {
			resource = "embed/index.html"
			b, err = res.ReadFile(resource)
			if err != nil {
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte("Not found"))
				return
			}
		}
	}

	var mimeType string

	extension := filepath.Ext(resource)
	switch extension {
	case ".css":
		mimeType = "text/css"
	case ".js":
		mimeType = "text/javascript"
	case ".png":
		mimeType = "image/png"
	case ".map":
		mimeType = "application/json"
	default:
		mimeType = "text/html"
	}

	w.Header().Set("Content-Type", mimeType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}

func (s *Stat) printStatisticsHTML(p io.Writer, root *Stat, keysParam string) {
	fmt.Fprintf(p, "<html><head>\r\n")
	fmt.Fprintf(p, "<title>\r\n")
	fmt.Fprintf(p, "GoCore Statistics\r\n")
	fmt.Fprintf(p, "</title>\r\n")
	fmt.Fprintf(p, "<script type='text/javascript' src='https://cdnjs.cloudflare.com/ajax/libs/jquery/1.3.2/jquery.min.js'></script>")
	fmt.Fprintf(p, "<script type='text/javascript' src='https://cdnjs.cloudflare.com/ajax/libs/jquery.tablesorter/2.31.3/js/jquery.tablesorter.min.js'></script>")
	fmt.Fprintf(p, "<script type='text/javascript' src='/js/chili-1.8b.js'></script>")
	fmt.Fprintf(p, "<link rel='stylesheet' href='/css/statistics.css' type='text/css' media='print, projection, screen' />")
	fmt.Fprintf(p, "<script type='text/javascript'>\r\n")

	fmt.Fprintf(p, "$(document).ready(function() \r\n")
	fmt.Fprintf(p, "{ \r\n")
	fmt.Fprintf(p, "$('#myTable').tablesorter( {\r\n")
	fmt.Fprintf(p, "sortList: [[1,1]],\r\n")
	fmt.Fprintf(p, "debug: false,\r\n")
	fmt.Fprintf(p, "widgets: ['zebra'],\r\n")
	fmt.Fprintf(p, "headers: {\r\n")
	fmt.Fprintf(p, "0: {sorter: 'text'},\r\n")
	fmt.Fprintf(p, "1: {sorter: 'timings'},\r\n")
	fmt.Fprintf(p, "2: {sorter: 'number'},\r\n")
	fmt.Fprintf(p, "3: {sorter: 'timings'},\r\n")
	fmt.Fprintf(p, "4: {sorter: 'timings'},\r\n")
	fmt.Fprintf(p, "5: {sorter: 'timings'},\r\n")
	fmt.Fprintf(p, "6: {sorter: 'timings'},\r\n")
	fmt.Fprintf(p, "7: {sorter: 'timings'},\r\n")
	fmt.Fprintf(p, "8: {sorter: 'usLongDate'},\r\n")
	fmt.Fprintf(p, "9: {sorter: 'usLongDate'}\r\n")

	fmt.Fprintf(p, "}\r\n")
	fmt.Fprintf(p, "} )\r\n")

	fmt.Fprintf(p, "} \r\n")
	fmt.Fprintf(p, ")  \r\n")
	fmt.Fprintf(p, "</script>\r\n")
	fmt.Fprintf(p, "</head>\r\n")
	fmt.Fprintf(p, "<body>\r\n")

	fmt.Fprint(p, "<table width='100%'>\r\n")
	fmt.Fprintf(p, "<tr>\r\n")
	// 		// Title
	fmt.Fprint(p, "<td style='vertical-align:middle;width:50%'>\r\n")
	fmt.Fprintf(p, "<h1>\r\n")
	fmt.Fprintf(p, "GoCore Statistics\r\n")
	fmt.Fprintf(p, "</h1>\r\n")
	fmt.Fprintf(p, "</td>\r\n")
	// 		// New button
	fmt.Fprint(p, "<td align='right' style='vertical-align:middle;width:50%' >\r\n")
	fmt.Fprintf(p, "<form border='0' cellpadding='0' action='reset' method='get'>\r\n")
	// 		// Using location.replace here so that the history buffer is not messed up for going back a page.
	fmt.Fprintf(p, "<input type='button' value='Reset Statistics' onClick='window.location.replace(\"reset?key=%s\");'>\r\n", keysParam)
	fmt.Fprintf(p, "</form>\r\n")
	fmt.Fprintf(p, "</td>\r\n")
	fmt.Fprintf(p, "</tr>\r\n")
	fmt.Fprintf(p, "</table>\r\n")
	// 		// End of change to add a reset button

	fmt.Fprintf(p, "<table id='myTable' class='tablesorter' border='0' cellpadding='0' cellspacing='1'>\r\n")
	fmt.Fprintf(p, "<thead>\r\n")
	fmt.Fprintf(p, "<tr>\r\n")
	fmt.Fprintf(p, "<th>Item</th>\r\n")
	fmt.Fprintf(p, "<th align='right'>total</th>\r\n")
	fmt.Fprintf(p, "<th align='right'>count</th>\r\n")
	fmt.Fprintf(p, "<th align='right'>first</th>\r\n")
	fmt.Fprintf(p, "<th align='right'>last</th>\r\n")
	fmt.Fprintf(p, "<th align='right'>min</th>\r\n")
	fmt.Fprintf(p, "<th align='right'>max</th>\r\n")
	fmt.Fprintf(p, "<th align='right'>average</th>\r\n")
	fmt.Fprintf(p, "<th>first run</th>\r\n")
	fmt.Fprintf(p, "<th>last run</th>\r\n")
	fmt.Fprintf(p, "</tr>\r\n")
	fmt.Fprintf(p, "</thead>\r\n")

	fmt.Fprintf(p, "<tbody>\r\n")

	item := root

	if item.children == nil {
		item.children = make(map[string]*Stat)
	}

	var keys []string
	if keysParam != "" {
		keys = strings.Split(keysParam, ",")
		keysParam += ","
	} else {
		keysParam = ""
	}

	for _, key := range keys {
		item = item.getChild(key)
		if item == nil {
			return
		}
	}

	now := time.Now().UTC()

	fmt.Fprintf(p, "<h2>Server started: %s [%s ago]</h2>\r\n", initTime.Format("2006-01-02 15:04:05.000"), utils.HumanTimeUnit(time.Since(initTime)))

	for key, item := range item.children {
		item.mu.RLock()

		fmt.Fprintf(p, "<tr>\r\n")
		if len(item.children) > 0 {
			// childKey := keysParam + key
			fmt.Fprintf(p, "<td><a href='/stats?key=%s'>%s</a></td>\r\n", keysParam+key, key)
		} else {
			fmt.Fprintf(p, "<td>%s</td>\r\n", key)
		}

		if item.hideTotal {
			fmt.Fprintf(p, "<td></td>\r\n")
		} else {
			fmt.Fprintf(p, "<td align='right'>%s</td>\r\n", utils.HumanTimeUnit(item.totalDuration))
		}

		fmt.Fprintf(p, "<td align='right'>%s</td>\r\n", addThousandsOperator(item.count))
		fmt.Fprintf(p, "<td align='right'>%s</td>\r\n", utils.HumanTimeUnit(item.firstDuration))
		fmt.Fprintf(p, "<td align='right'>%s</td>\r\n", utils.HumanTimeUnit(item.lastDuration))
		fmt.Fprintf(p, "<td align='right'>%s</td>\r\n", utils.HumanTimeUnit(item.minDuration))
		fmt.Fprintf(p, "<td align='right'>%s</td>\r\n", utils.HumanTimeUnit(item.maxDuration))
		fmt.Fprintf(p, "<td align='right'>%s</td>\r\n", utils.HumanTimeUnit(item.average()))
		fmt.Fprintf(p, "<td>%s</td>\r\n", item.firstTime.Format("2006-01-02 15:04:05.000"))
		fmt.Fprintf(p, "<td>%s</td>\r\n", item.lastTime.Format("2006-01-02 15:04:05.000"))
		fmt.Fprintf(p, "</tr>\r\n")

		item.mu.RUnlock()
	}

	fmt.Fprintf(p, "</tbody>\r\n")

	fmt.Fprintf(p, "</table>\r\n")
	fmt.Fprintf(p, "<p>Report time: %s</p>\r\n", now.Format("2006-01-02 15:04:05.000"))
	fmt.Fprintf(p, "<div align='right'>")
	fmt.Fprintf(p, "<form>\r\n\r\n")
	fmt.Fprintf(p, "<input type='button' value='  Back  ' onClick='history.go(-1)'>\r\n")
	fmt.Fprintf(p, "</form>\r\n")
	fmt.Fprintf(p, "</div>\r\n")
	fmt.Fprintf(p, "</body></html>\r\n")

}

func addThousandsOperator(num int64) string {
	p := message.NewPrinter(language.English)
	return p.Sprintf("%d\n", num)
}
