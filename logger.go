package gocore

import (
	"bufio"
	"errors"
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
	sockets map[net.Conn]string
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
	socketDIR    string
	outputFormat string
)

func init() {
	loggers = make(map[string]*Logger)

	socketDIR, _ = Config().Get("socketDIR")
	if socketDIR == "" {
		socketDIR = "/tmp/gocore"
	}
	err := os.MkdirAll(socketDIR, os.ModePerm)
	if err != nil {
		log.Printf("ERROR: Unable to make socket directory %s: %+v", socketDIR, err)
	}

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
				sockets: make(map[net.Conn]string),
			},
		},
		showTimestamp: Config().GetBool("logger_show_timestamps", true),
	}

	if !logger.showTimestamp {
		log.SetFlags(0)
	}

	// Run a listener on a Unix socket
	go func() {
		n := fmt.Sprintf("%s/%s.sock", socketDIR, strings.ToUpper(packageNameStr))

		// Remove the file if it exists...
		os.Remove(n)

		ln, err := net.Listen("unix", n)
		if err != nil {
			logger.Fatalf("LOGGER: listen error: %+v", err)
		}
		defer ln.Close()

		defer os.Remove(n)

		// Add the socket so we can close it down when Fatal or Panic are called
		logger.conf.socket = ln

		if Config().GetBool("logger_show_socket_info", false) {
			logger.Infof("Socket created. Connect with 'nc -U %s'", n)
		}
		for {
			fd, err := ln.Accept()
			if err != nil && Config().GetBool("logger_show_socket_info", false) {
				logger.Warnf("Accept error: %+v", err)
				return
			}

			logger.handleIncomingMessage(fd)
		}

	}()

	loggers[packageNameStr] = logger

	return logger
}

func (l *Logger) write(w io.Writer, s string) error {
	if w == nil {
		l.Errorf("Writing client error: %+v", "Writer is nil")
		return errors.New("writer is nil")
	}

	_, err := w.Write([]byte(s))
	if err != nil {
		l.Errorf("Writing client error: %+v", err)
	}
	return err
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

func (l *Logger) handleIncomingMessage(conn net.Conn) {
	l.welcome(conn)
	go func() {
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			cmd := scanner.Text()
			s, err := utils.SplitArgs(cmd)
			if err != nil {
				if err := l.write(conn, fmt.Sprintf("  Cannot split command: %v\n\n", err)); err != nil {
					return
				}
			}

			switch s[0] {
			case "config":
				l.handleConfig(s, conn)
			case "trace":
				l.handleDebugAndTrace("TRACE", s, conn)
			case "loglevel":
				l.handleLogLevel(s, conn)
			case "sample":
				l.handleSample(s, conn)
			case "status":
				l.sendStatus(conn)
			case "quit":
				conn.Close()
				return
			case "help":
				l.help(conn)
			case "":

			default:
				if err := l.write(conn, fmt.Sprintf("  Command not found: %s\n\n", cmd)); err != nil {
					return
				}
			}
		}
	}()
}

func (l *Logger) handleSample(r []string, conn net.Conn) {

	if len(r) <= 1 {
		_ = l.write(conn, "  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
		return
	}

	switch r[1] {
	case "list":
		if len(r) != 2 {
			_ = l.write(conn, "  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
			return
		}

		if len(l.conf.samplers) == 0 {
			err := l.write(conn, "  No running samples.\n\n")
			if err != nil {
				return
			}
		} else {
			s := ""
			for _, sampler := range l.conf.samplers {
				s += fmt.Sprintf("  %s\n", sampler)
			}
			s += "\n"

			err := l.write(conn, s)
			if err != nil {
				break
			}
		}

	case "start":
		if len(r) < 4 || len(r) > 5 {
			_ = l.write(conn, "  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
			return
		}

		id := r[2]

		filename := r[3]

		regex := ""
		if len(r) == 5 {
			regex = r[4]
		}

		s, err := sampler.New(id, filename, regex)
		if err != nil {
			_ = l.write(conn, fmt.Sprintf("  Could not create sampler [%v].\n\n", err))
			return
		}

		l.conf.mu.Lock()
		l.conf.samplers = append(l.conf.samplers, s)
		l.conf.mu.Unlock()

		l.sendStatus(conn)

	case "stop":
		if len(r) != 3 {
			_ = l.write(conn, "  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
			return
		}

		id := r[2]

		l.conf.mu.Lock()
		for i, sampler := range l.conf.samplers {
			if sampler.ID == id {
				sampler.Stop()
				l.conf.samplers = append(l.conf.samplers[:i], l.conf.samplers[i+1:]...)
				break // Only close the first sampler with this ID in case there are more than one with the same ID
			}
		}
		l.conf.mu.Unlock()

		l.sendStatus(conn)
	default:
		_ = l.write(conn, "  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
		return
	}
}

func (l *Logger) handleConfig(r []string, conn net.Conn) {
	if len(r) <= 1 {
		_ = l.write(conn, "  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
		return
	}

	switch r[1] {
	case "requested":
		requested := Config().Requested()
		_ = l.write(conn, fmt.Sprintf("\n%s\n\n", requested))

	case "show":
		stats := Config().Stats()
		_ = l.write(conn, stats+"\n\n")

	case "get":
		value, ok := Config().Get(r[2])
		if !ok {
			_ = l.write(conn, "  Not set\n\n")
		} else {
			_ = l.write(conn, fmt.Sprintf("  %s=%s\n\n", r[2], value))
		}

	case "set":
		oldValue := Config().Set(r[2], r[3])

		if oldValue == r[3] {
			_ = l.write(conn, "  No change\n\n")
		} else if oldValue == "" {
			_ = l.write(conn, fmt.Sprintf("  Created new setting: %s=%s\n\n", r[2], r[3]))
		} else {
			_ = l.write(conn, fmt.Sprintf("  Updated setting: %s %s -> %s\n\n", r[2], oldValue, r[3]))
		}
	case "unset":
		oldValue := Config().Unset(r[2])
		if oldValue == "" {
			_ = l.write(conn, "  No change\n\n")
		} else {
			_ = l.write(conn, fmt.Sprintf("  Removed setting: %s=%s\n\n", r[2], oldValue))
		}
	}
}

func (l *Logger) handleLogLevel(r []string, conn net.Conn) {
	ll := NewLogLevelFromString(r[1])
	l.setLogLevel(ll)
}

func (l *Logger) handleDebugAndTrace(context string, r []string, conn net.Conn) {
	if len(r) <= 1 {
		_ = l.write(conn, "  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
		return
	}

	switch r[1] {
	case "off":
		if len(r) != 2 {
			_ = l.write(conn, "  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
			return
		}
		switch context {
		case "TRACE":
			l.conf.mu.Lock()
			delete(l.conf.trace.sockets, conn)
			l.conf.mu.Unlock()
			l.sendStatus(conn)
		default:
			_ = l.write(conn, "Invalid context'\n")
		}

	case "on":
		if len(r) > 3 {
			_ = l.write(conn, "  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
			return
		}

		reg := ""
		if len(r) == 3 {
			reg = r[2]
		}

		switch context {
		case "TRACE":
			l.conf.mu.Lock()
			l.conf.trace.sockets[conn] = reg
			l.conf.mu.Unlock()
			l.sendStatus(conn)
		default:
			_ = l.write(conn, "Invalid context'\n")
		}

	default:
		_ = l.write(conn, "  Second parameter must be 'on' or 'off'\n\n")
	}
}

func (l *Logger) sendStatus(conn net.Conn) {
	l.conf.mu.RLock()
	defer l.conf.mu.RUnlock()

	res := fmt.Sprintf("  LOG LEVEL is %s\n", l.conf.logLevel)

	if regex, ok := l.conf.trace.sockets[conn]; ok {
		if regex != "" {
			res += fmt.Sprintf("  TRACE is ON filtered by a regex of %q\n", regex)
		} else {
			res += "  TRACE is ON with no filter\n"
		}
	} else {
		res += "  TRACE is OFF\n"
	}

	res += fmt.Sprintf("  %d SAMPLES running\n", len(l.conf.samplers))

	res += "\n"

	_ = l.write(conn, res)
}

func (l *Logger) help(conn net.Conn) {
	type command struct {
		cmd         string
		description string
	}

	cmds := []command{
		{
			cmd:         "loglevel [DEBUG | INFO | WARN | ERROR] ",
			description: "Set loglevel for this instance",
		},
		{
			cmd:         "trace [on {regex} | off] ",
			description: "Turn on/off trace mode with an optional Regex pattern",
		},
		{
			cmd:         "sample [start <id> <filename> {regex} | stop <id> | list] ",
			description: "Turn on/off samplers mode with an optional Regex pattern",
		},
		{
			cmd:         "config [get <key> | set <key> <value> | unset <key> | show | requested ] ",
			description: "Manage settings dynamically and show current settings or requested settings",
		},
		{
			cmd:         "status",
			description: "Shows the status of debug",
		},
		{
			cmd:         "help",
			description: "Shows the available commands",
		},
		{
			cmd:         "quit",
			description: "Terminates this session",
		},
	}

	res := ""
	for _, c := range cmds {
		res += fmt.Sprintf("  %s (%s)\n", c.cmd, c.description)
	}
	res += "\n"

	_ = l.write(conn, res)
}

func (l *Logger) welcome(conn net.Conn) {

	res := "Runtime logger controller.\n-------------------------\nType help for a list of available commands.\n\n"
	_ = l.write(conn, res)
}
