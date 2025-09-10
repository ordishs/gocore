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
	"sync/atomic"
	"time"

	"github.com/ordishs/gocore/utils"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var (
	//go:embed all:embed/*

	embeddedFS embed.FS

	statPrefix string

	registerOnce sync.Once

	initTime = time.Now().UTC()
	RootStat = &Stat{
		key:                "root",
		ignoreChildUpdates: true,
	}

	reportedTimeThresholdStr string
	reportedTimeThreshold    time.Duration

	queue *lockFreeQueue
)

func init() {
	reportedTimeThresholdStr, _ = Config().Get("gocore_stats_reported_time_threshold", "5m")

	var err error
	reportedTimeThreshold, err = time.ParseDuration(reportedTimeThresholdStr)
	if err != nil {
		reportedTimeThreshold = 5 * time.Minute
	}

	statPrefix, _ = Config().Get("stats_prefix", "/") // Use the desired default prefix or configuration key
	if !strings.HasPrefix(statPrefix, "/") {
		statPrefix = "/" + statPrefix
	}
	if !strings.HasSuffix(statPrefix, "/") {
		statPrefix += "/"
	}

	queue = newLockFreeQueue()

	go func() {
		for {
			s := queue.dequeue()
			if s == nil {
				time.Sleep(1 * time.Millisecond)
				continue
			}

			s.stat.processTime(s.now, s.duration)
		}

	}()
}

func GetStatPrefix() string {
	return statPrefix
}

// Stat comment
type Stat struct {
	mu                 sync.RWMutex
	key                string
	parent             *Stat
	childMap           sync.Map
	rangeLower         int
	rangeUpper         int
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
	newStat := &Stat{
		key:    key,
		parent: s,
	}

	if len(options) > 0 {
		newStat.ignoreChildUpdates = options[0]
	}

	stat, loaded := s.childMap.LoadOrStore(key, newStat)
	if loaded {
		// If the stat was already in the map, it's returned as is.
		return stat.(*Stat)
	}

	// If the stat was not in the map, the newly created stat is returned.
	return newStat
}

func (s *Stat) AddRanges(ranges ...int) *Stat {
	if len(ranges) > 0 {
		// sort the ranges
		for i := 0; i < len(ranges); i++ {
			for j := i + 1; j < len(ranges); j++ {
				if ranges[i] > ranges[j] {
					ranges[i], ranges[j] = ranges[j], ranges[i]
				}
			}
		}

		for i := 0; i < len(ranges); i++ {
			// if i == 0 {
			// 	s.childMap.LoadOrStore(fmt.Sprintf("< %s", addThousandsOperatorTrim(ranges[i])), &Stat{rangeLower: 0, rangeUpper: ranges[i], parent: s})
			// } else
			if i == len(ranges)-1 {
				s.childMap.LoadOrStore(fmt.Sprintf("%s -", addThousandsOperatorTrim(ranges[i])), &Stat{rangeLower: ranges[i], rangeUpper: -1, parent: s})
			} else {
				s.childMap.LoadOrStore(fmt.Sprintf("%s - %s", addThousandsOperatorTrim(ranges[i]), addThousandsOperatorTrim(ranges[i+1])), &Stat{rangeLower: ranges[i], rangeUpper: ranges[i+1], parent: s})
			}
		}
	}
	return s
}

func (s *Stat) HideTotal(b bool) {
	s.hideTotal = b
}

func (s *Stat) getChild(key string) *Stat {
	if stat, ok := s.childMap.Load(key); ok {
		return stat.(*Stat)
	}

	return nil
}

func (s *Stat) processTime(now time.Time, duration time.Duration) {
	if duration > reportedTimeThreshold {
		log.Printf("Stat: time for %s is greater than %s", s.key, reportedTimeThresholdStr)
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

func (s *Stat) AddTimeForRange(startTime time.Time, sampleSize int) time.Time {
	now := time.Now().UTC()

	duration := now.Sub(startTime)

	if duration < 0 {
		log.Printf("%s: startTime is in the future", s.key)
		return now
	}

	var found bool

	// Work out which bucket this time fits into
	s.childMap.Range(func(_, child interface{}) bool {
		if child.(*Stat).rangeLower <= sampleSize && (child.(*Stat).rangeUpper == -1 || sampleSize < child.(*Stat).rangeUpper) {
			child.(*Stat).processTime(now, duration)
			found = true
			return false // Stop iterating
		}

		return true // Keep iterating
	})

	if !found {
		log.Printf("%s: sampleSize %d does not fit into any range", s.key, sampleSize)
	}

	return now
}

// AddTime comment
func (s *Stat) AddTime(startTime time.Time) time.Time {
	now := time.Now().UTC()

	duration := now.Sub(startTime)

	if duration < 0 {
		log.Printf("%s: startTime is in the future", s.key)
		return now
	}

	queue.enqueue(&statItem{
		stat:     s,
		now:      now,
		duration: duration,
	})

	return now
}

func (s *Stat) reset() {
	s.mu.Lock()

	s.firstDuration = 0
	s.lastDuration = 0
	s.minDuration = 0
	s.maxDuration = 0
	s.totalDuration = 0
	s.count = 0
	s.firstTime = time.Time{}
	s.lastTime = time.Time{}

	s.mu.Unlock()

	s.childMap.Range(func(_, value interface{}) bool {
		value.(*Stat).reset()
		return true
	})
}

func CurrentTime() time.Time {
	return time.Now().UTC()
}

func average(totalDuration time.Duration, count int64) time.Duration {
	if count == 0 {
		return 0
	}

	return time.Duration(totalDuration.Nanoseconds() / count)
}

// func (s *Stat) String() string {
// 	s.mu.RLock()
// 	defer s.mu.RUnlock()

// 	return fmt.Sprintf("%s (%t): %s (%d)", s.key, s.ignoreChildUpdates, utils.HumanTime(s.totalDuration), s.count)
// }

func RegisterStatsHandlers(mux ...*http.ServeMux) {
	registerOnce.Do(func() {
		var muxes []*http.ServeMux

		if len(mux) == 0 {
			muxes = []*http.ServeMux{http.DefaultServeMux}
		} else {
			muxes = mux
		}

		for _, m := range muxes {
			m.HandleFunc(statPrefix+"stats", HandleStats)
			m.HandleFunc(statPrefix+"reset", ResetStats)
			m.HandleFunc(statPrefix+"", HandleOther)
		}
	})
}

func StartStatsServer(addr string) {
	logger := Log("stats")

	RegisterStatsHandlers()

	logger.Infof("Starting StatsServer on http://%s%sstats", addr, statPrefix)
	var err = http.ListenAndServe(addr, nil)

	if err != nil {
		logger.Panicf("Server failed starting. Error: %s", err)
	}
}

func HandleStats(w http.ResponseWriter, r *http.Request) {
	keysParam := r.URL.Query().Get("key")

	RootStat.printStatisticsHTML(w, RootStat, keysParam)
}

func ResetStats(w http.ResponseWriter, r *http.Request) {
	keysParam := r.URL.Query().Get("key")
	item := RootStat

	if keysParam != "" {
		for _, key := range strings.Split(keysParam, ",") {
			item = item.getChild(key)
		}
	}

	item.reset()
	http.Redirect(w, r, statPrefix+"stats", http.StatusSeeOther)
}

func HandleOther(w http.ResponseWriter, r *http.Request) {
	var resource string

	path := r.URL.Path

	trimmedPath := strings.TrimPrefix(path, statPrefix)
	if trimmedPath == "/" || trimmedPath == "" {
		resource = "embed/index.html"
	} else {
		resource = fmt.Sprintf("embed/%s", trimmedPath)
	}

	b, err := embeddedFS.ReadFile(resource)
	if err != nil {
		// Just in case we're missing the /index.html, add it and try again...
		resource += "/index.html"
		b, err = embeddedFS.ReadFile(resource)
		if err != nil {
			resource = "embed/index.html"
			b, err = embeddedFS.ReadFile(resource)
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

	fmt.Fprintf(p, `
	<html>
		<head>
			<title>
			GoCore Statistics
			</title>
			<script src="https://cdnjs.cloudflare.com/ajax/libs/jquery/1.4.3/jquery.min.js" integrity="sha512-xqRHwg8Pg0JQ+nne5mBy3SGrGDihpsr5UYuMgIcVj1SMfSKrRJNvu7tFitaK70xDpSsBBIVpTcTGXnmx7/Q2xw==" crossorigin="anonymous" referrerpolicy="no-referrer"></script>
			<script src="https://cdnjs.cloudflare.com/ajax/libs/jquery.tablesorter/2.31.3/js/jquery.tablesorter.min.js" integrity="sha512-qzgd5cYSZcosqpzpn7zF2ZId8f/8CHmFKZ8j7mU4OUXTNRd5g+ZHBPsgKEwoqxCtdQvExE5LprwwPAgoicguNg==" crossorigin="anonymous" referrerpolicy="no-referrer"></script>
			<script src="https://cdnjs.cloudflare.com/ajax/libs/jquery.tablesorter/2.31.3/js/jquery.tablesorter.widgets.min.js" integrity="sha512-dj/9K5GRIEZu+Igm9tC16XPOTz0RdPk9FGxfZxShWf65JJNU2TjbElGjuOo3EhwAJRPhJxwEJ5b+/Ouo+VqZdQ==" crossorigin="anonymous" referrerpolicy="no-referrer"></script>
			<script type='text/javascript' src='%sjs/chili-1.8b.js'></script>
			<link rel='stylesheet' href='%scss/statistics.css' type='text/css' media='print, projection, screen' />

			<script type='text/javascript'>

				function convertToNanoseconds(duration) {
					const timeUnits = {
						d: 24 * 60 * 60 * 1e9, // days to nanoseconds
						h: 60 * 60 * 1e9,      // hours to nanoseconds
						m: 60 * 1e9,           // minutes to nanoseconds
						s: 1e9,                // seconds to nanoseconds
						ms: 1e6,               // milliseconds to nanoseconds
						µs: 1e3,               // microseconds to nanoseconds
						ns: 1                  // nanoseconds
					};

					const regex = /(\d+(\.\d+)?)(d|h|ms|ns|m|µs|s)/g;
			
					let totalNanoseconds = 0;
			
					const matches = duration.matchAll(regex);
				
					for (const match of matches) {
							const value = parseFloat(match[1]);
							const timeUnit = match[3];
							// console.log(value, timeUnit, value * (timeUnits[timeUnit] || 0))
							totalNanoseconds += value * (timeUnits[timeUnit] || 0);
					}
					
					// console.log(duration, totalNanoseconds)
					return totalNanoseconds;
				}
			
				$(document).ready(function() {
					$.tablesorter.addParser({
						id: 'timings',
						is: function(s) {
							// Return false so this parser is not auto detected
							return false;
						},
						format: function(s) {
							return convertToNanoseconds(s);
						},
						type: 'numeric'
					});

					$('#myTable').tablesorter({ 
						sortList: [[1,1]], 
						debug: false, 
						widgets: ['zebra', 'saveSort'], 
						headers: { 
							0: {sorter: 'text'}, 
							1: {sorter: 'number'}, 
							2: {sorter: 'timings'}, 
							3: {sorter: 'timings'}, 
							4: {sorter: 'timings'}, 
							5: {sorter: 'timings'}, 
							6: {sorter: 'timings'}, 
							7: {sorter: 'timings'}, 
							8: {sorter: 'usLongDate'}, 
							9: {sorter: 'usLongDate'} 
						}, 
						widgetOptions: { 
							saveSort: true 
						} 
					}); 
				})  
				</script>
			</head>
	`, statPrefix, statPrefix)

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
	fmt.Fprintf(p, "<form border='0' cellpadding='0' action='%sreset' method='get'>\r\n", statPrefix)
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
	fmt.Fprintf(p, "<th align='right'>count</th>\r\n")
	fmt.Fprintf(p, "<th align='right'>average</th>\r\n")
	fmt.Fprintf(p, "<th align='right'>first</th>\r\n")
	fmt.Fprintf(p, "<th align='right'>last</th>\r\n")
	fmt.Fprintf(p, "<th align='right'>min</th>\r\n")
	fmt.Fprintf(p, "<th align='right'>max</th>\r\n")
	fmt.Fprintf(p, "<th align='right'>total</th>\r\n")
	fmt.Fprintf(p, "<th>first run</th>\r\n")
	fmt.Fprintf(p, "<th>last run</th>\r\n")
	fmt.Fprintf(p, "</tr>\r\n")
	fmt.Fprintf(p, "</thead>\r\n")

	fmt.Fprintf(p, "<tbody>\r\n")

	item := root

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

	fmt.Fprintf(p, "<h2>Server started: %s [%s ago]</h2>\r\n", initTime.Format("2006-01-02 15:04:05.000"), utils.HumanTimeUnitHTML(time.Since(initTime)))

	item.childMap.Range(func(keyI, itemI interface{}) bool {
		item := itemI.(*Stat)

		item.mu.RLock()

		fmt.Fprintf(p, "<tr>\r\n")

		if hasChildren(&item.childMap) {
			// childKey := keysParam + key
			fmt.Fprintf(p, "<td><a href='%sstats?key=%s'>%s</a></td>\r\n", statPrefix, keysParam+keyI.(string), keyI.(string))
		} else {
			fmt.Fprintf(p, "<td>%s</td>\r\n", keyI.(string))
		}

		fmt.Fprintf(p, "<td align='right'>%s</td>\r\n", addThousandsOperator(item.count))
		fmt.Fprintf(p, "<td align='right'>%s</td>\r\n", utils.HumanTimeUnitHTML(average(item.totalDuration, item.count)))
		fmt.Fprintf(p, "<td align='right'>%s</td>\r\n", utils.HumanTimeUnitHTML(item.firstDuration))
		fmt.Fprintf(p, "<td align='right'>%s</td>\r\n", utils.HumanTimeUnitHTML(item.lastDuration))
		fmt.Fprintf(p, "<td align='right'>%s</td>\r\n", utils.HumanTimeUnitHTML(item.minDuration))
		fmt.Fprintf(p, "<td align='right'>%s</td>\r\n", utils.HumanTimeUnitHTML(item.maxDuration))

		if item.hideTotal {
			fmt.Fprintf(p, "<td></td>\r\n")
		} else {
			fmt.Fprintf(p, "<td align='right'>%s</td>\r\n", utils.HumanTimeUnitHTML(item.totalDuration))
		}

		fmt.Fprintf(p, "<td>%s</td>\r\n", item.firstTime.Format("2006-01-02 15:04:05.000"))
		fmt.Fprintf(p, "<td>%s</td>\r\n", item.lastTime.Format("2006-01-02 15:04:05.000"))
		fmt.Fprintf(p, "</tr>\r\n")

		item.mu.RUnlock()

		return true // keep iterating
	})

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

func addThousandsOperatorTrim(num int) string {
	p := message.NewPrinter(language.English)
	return p.Sprintf("%d", num)
}

func hasChildren(m *sync.Map) bool {
	var hasChildren bool

	m.Range(func(_, _ interface{}) bool {
		hasChildren = true
		return false // stop iteration immediately after finding the first item
	})

	return hasChildren
}

type statItem struct {
	stat     *Stat
	now      time.Time
	duration time.Duration
	next     atomic.Pointer[statItem]
}

type lockFreeQueue struct {
	head         atomic.Pointer[statItem]
	tail         *statItem
	previousTail *statItem
	queueLength  atomic.Int64
}

// NewLockFreeQueue creates and initializes a LockFreeQueue
func newLockFreeQueue() *lockFreeQueue {
	firstTail := &statItem{}
	lf := &lockFreeQueue{
		head:         atomic.Pointer[statItem]{},
		tail:         firstTail,
		previousTail: firstTail,
	}

	lf.head.Store(nil)

	return lf
}

// Enqueue adds a series of Request to the queue
// enqueue is thread safe, it uses atomic operations to add to the queue
func (q *lockFreeQueue) enqueue(v *statItem) {
	prev := q.head.Swap(v)
	if prev == nil {
		q.tail.next.Store(v)
		return
	}
	prev.next.Store(v)
}

// Dequeue removes a Request from the queue
// dequeue is not thread safe, it should only be called from a single thread !!!
func (q *lockFreeQueue) dequeue() *statItem {
	next := q.tail.next.Load()

	if next == nil || next == q.previousTail {
		return nil
	}

	q.tail = next
	q.previousTail = next
	q.queueLength.Add(-1)
	return next
}
