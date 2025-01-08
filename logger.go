package gocore

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/mgutz/ansi"
	"github.com/ordishs/gocore/sampler"
	"github.com/ordishs/gocore/utils"
)

// TraceSettings Comment
type traceSettings struct {
	// String will be a Regex expression for the relevant Conn
	sockets map[io.ReadWriteCloser]string
}

type logLevel int

const (
	DEBUG logLevel = iota
	INFO
	WARN
	ERROR
	FATAL
	PANIC
)

var (
	logLevel_name = map[int]string{
		0: "DEBUG",
		1: "INFO",
		2: "WARN",
		3: "ERROR",
		4: "FATAL",
		5: "PANIC",
	}

	logLevel_value = map[string]int{
		"DEBUG": 0,
		"INFO":  1,
		"WARN":  2,
		"ERROR": 3,
		"FATAL": 4,
		"PANIC": 5,
	}
)

func (ll logLevel) String() string {
	return logLevel_name[int(ll)]
}

func NewLogLevelFromString(s string) logLevel {
	s = strings.ToUpper(s)
	return logLevel(logLevel_value[s])
}

// LoggerConfig comment
type loggerConfig struct {
	mu       *sync.RWMutex
	socket   net.Listener
	logLevel logLevel
	trace    traceSettings
	samplers []*sampler.Sampler
}

// Logger comment
type Logger struct {
	packageName   string
	colour        bool
	conf          loggerConfig
	showTimestamp bool
}

var (
	mu           sync.RWMutex
	loggers      map[string]*Logger
	outputFormat string
)

func init() {
	loggers = make(map[string]*Logger)

	outputFormat, _ = Config().Get("logger_output_format", "| %-20s| %-5s| %s |")
}

func Log(packageNameStr string, logLevelOption ...logLevel) *Logger {
	mu.Lock()
	defer mu.Unlock()

	logger, found := loggers[packageNameStr]
	if found {
		return logger
	}

	var ll logLevel

	if len(logLevelOption) > 0 {
		ll = logLevelOption[0]
	} else {
		logLevelSetting, _ := Config().Get("logLevel", "INFO")
		ll = NewLogLevelFromString(logLevelSetting)
	}

	logger = &Logger{
		packageName: packageNameStr,
		colour:      true,
		conf: loggerConfig{
			mu:       new(sync.RWMutex),
			logLevel: ll,
			trace: traceSettings{
				sockets: make(map[io.ReadWriteCloser]string),
			},
		},
		showTimestamp: Config().GetBool("logger_show_timestamps", true),
	}

	if !logger.showTimestamp {
		log.SetFlags(0)
	}

	// Start the Unix socket listener
	if err := StartSocketListener(logger); err != nil {
		logger.Fatalf("LOGGER: %v", err)
	}

	loggers[packageNameStr] = logger

	return logger
}

// Debug Comment
func (l *Logger) Debug(args ...interface{}) {
	msg := ""
	l.output(DEBUG, "blue", msg, args...)
}

// Debugf Comment
func (l *Logger) Debugf(msg string, args ...interface{}) {
	l.output(DEBUG, "blue", msg, args...)
}

// Info comment
func (l *Logger) Info(args ...interface{}) {
	msg := ""
	l.output(INFO, "green", msg, args...)
}

// Infof comment
func (l *Logger) Infof(msg string, args ...interface{}) {
	l.output(INFO, "green", msg, args...)
}

// Warn comment
func (l *Logger) Warn(args ...interface{}) {
	msg := ""
	l.output(WARN, "yellow", msg, args...)
}

// Warnf comment
func (l *Logger) Warnf(msg string, args ...interface{}) {
	l.output(WARN, "yellow", msg, args...)
}

// Error comment
func (l *Logger) Error(args ...interface{}) {
	msg := ""
	l.output(ERROR, "red", msg, args...)
}

// Errorf comment
func (l *Logger) Errorf(msg string, args ...interface{}) {
	l.output(ERROR, "red", msg, args...)
}

// ErrorWithStack comment
func (l *Logger) ErrorWithStack(msg string, args ...interface{}) {
	args = append(args, l.getStack())
	msg = msg + "\n%s"
	l.output(ERROR, "red", msg, args...)
}

// Fatal Comment
func (l *Logger) Fatal(args ...interface{}) {
	msg := ""
	l.output(FATAL, "cyan", msg, args...)
	if l.conf.socket != nil {
		l.conf.socket.Close()
	}
	os.Exit(1)
}

// Fatalf Comment
func (l *Logger) Fatalf(msg string, args ...interface{}) {
	l.output(FATAL, "cyan", fmt.Sprintf(msg, args...))
	if l.conf.socket != nil {
		l.conf.socket.Close()
	}
	os.Exit(1)
}

// Panic Comment
func (l *Logger) Panic(args ...interface{}) {
	msg := ""
	l.output(PANIC, "magenta", msg, args...)
	if l.conf.socket != nil {
		l.conf.socket.Close()
	}
	log.Panic(args...)
}

// Panicf Comment
func (l *Logger) Panicf(msg string, args ...interface{}) {
	l.output(PANIC, "magenta", msg, args...)
	if l.conf.socket != nil {
		l.conf.socket.Close()
	}
	log.Panicf(msg, args...)
}

func (l *Logger) output(ll logLevel, colour, msg string, args ...interface{}) {
	print, canReturn := l.loggingNecessary(ll)
	if canReturn {
		return
	}

	// We want the level to be 5 chars.  Append spaces if necessary
	level := ll.String()
	for i := len(level); i < 5; i++ {
		level += " "
	}

	if l.colour && colour != "" {
		level = ansi.Color(level, colour)
	}

	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "???"
		line = 0
	}

	// See if this filename includes the jenkins path and concat if necessary
	parts := strings.Split(file, "/")
	if len(parts) > 1 {
		file = parts[len(parts)-1]
	}

	fileLine := fmt.Sprintf("%s:%d", file, line)

	format := fmt.Sprintf(outputFormat, fileLine, l.packageName, level)
	if msg != "" {
		format = fmt.Sprintf(outputFormat+" %s", fileLine, l.packageName, level, msg)
	}

	if print {
		if msg != "" {
			log.Printf(format, args...)
		} else {
			m := []interface{}{format}
			m = append(m, args...)
			log.Println(m...)
		}
	}

	var s string

	if l.showTimestamp {
		s += time.Now().UTC().Format("2006-01-02 15:04:05.000 ")
	}

	s += fmt.Sprintf(format, args...)

	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}

	l.sendToTrace(s, level)

	l.sendToSample(s, level)
}

func (l *Logger) getStack() string {
	return strings.Join(strings.Split(string(debug.Stack()), "\n")[7:], "\n")
}

func (l *Logger) GetLogLevel() logLevel {
	l.conf.mu.RLock()
	defer l.conf.mu.RUnlock()
	return l.conf.logLevel
}

func (l *Logger) LogLevel() int {
	l.conf.mu.RLock()
	defer l.conf.mu.RUnlock()
	return int(l.conf.logLevel)
}

func (l *Logger) loggingNecessary(ll logLevel) (bool, bool) {
	l.conf.mu.RLock()
	defer l.conf.mu.RUnlock()

	print := ll >= l.conf.logLevel

	if len(l.conf.trace.sockets) > 0 {
		return print, false
	}

	if len(l.conf.samplers) > 0 {
		return print, false
	}

	return print, !print
}

func (l *Logger) setLogLevel(ll logLevel) {
	l.conf.mu.Lock()
	defer l.conf.mu.Unlock()
	l.conf.logLevel = ll
}

func (l *Logger) sendToTrace(s string, level string) {
	l.conf.mu.Lock()
	defer l.conf.mu.Unlock()

	for sock, r := range l.conf.trace.sockets {
		if utils.IsRegexMatch(r, s) || utils.IsRegexMatch(strings.ToLower(r), strings.ToLower(level)) {
			_, e := sock.Write([]byte(s))
			if e != nil {
				log.Println(ansi.Color(fmt.Sprintf("Writing client error: '%s'", e), "red"))
				delete(l.conf.trace.sockets, sock)
			}
		}
	}
}

func (l *Logger) sendToSample(s string, level string) {
	l.conf.mu.Lock()
	defer l.conf.mu.Unlock()

	for _, sampler := range l.conf.samplers {
		if utils.IsRegexMatch(sampler.Regex, s) || utils.IsRegexMatch(strings.ToLower(sampler.Regex), strings.ToLower(level)) {
			sampler.Write(s)
		}
	}
}
