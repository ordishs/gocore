package gocore

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mgutz/ansi"
	"github.com/ordishs/gocore/sampler"
	"github.com/ordishs/gocore/utils"
)

type debugSettings struct {
	enabled bool
	regex   string
}

// TraceSettings Comment
type traceSettings struct {
	// String will be a Regex expression for the relevant Conn
	sockets map[net.Conn]string
}

// LoggerConfig comment
type loggerConfig struct {
	mu       *sync.RWMutex
	socket   net.Listener
	debug    debugSettings
	trace    traceSettings
	samplers []*sampler.Sampler
}

// Logger comment
type Logger struct {
	packageName string
	colour      bool
	conf        loggerConfig
	infoLog     *log.Logger
	errorLog    *log.Logger
}

var (
	logger     *Logger
	loggerOnce sync.Once
	socketDIR  string
)

func init() {
	socketDIR, _ = Config().Get("socketDIR")
	if socketDIR == "" {
		socketDIR = "/tmp/gocore"
	}
	err := os.MkdirAll(socketDIR, os.ModePerm)
	if err != nil {
		log.Printf("ERROR: Unable to make socket directory %s: %+v", socketDIR, err)
	}
}

// Log comment
func Log(packageName string) *Logger {
	loggerOnce.Do(func() {
		logger = &Logger{
			packageName: packageName,
			colour:      true,
			conf: loggerConfig{
				mu: new(sync.RWMutex),
				trace: traceSettings{
					sockets: make(map[net.Conn]string),
				},
			},
			infoLog:  log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime|log.LUTC),
			errorLog: log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.LUTC|log.Llongfile),
		}

		SetPackageName(packageName)

		// Run a listener on a Unix socket
		go func() {
			n := fmt.Sprintf("%s/%s.sock", socketDIR, strings.ToUpper(packageName))

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

			logger.Infof("Socket created. Connect with 'nc -U %s'", n)

			ch := make(chan os.Signal, 1)
			signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

			logger.handleShutdown(ln, ch)

			for {
				fd, err := ln.Accept()
				if err != nil {
					logger.Warnf("Accept error: %+v", err)
					return
				}

				logger.handleIncomingMessage(fd)
			}

		}()

	})

	return logger
}

func (l *Logger) write(c io.Writer, s string) error {
	_, err := c.Write([]byte(s))
	if err != nil {
		l.Errorf("Writing client error: %+v", err)
	}
	return err
}

// Debug Comment
func (l *Logger) Debug(args ...interface{}) {
	msg := ""
	l.output("DEBUG", "blue", msg, args...)
}

// Debugf Comment
func (l *Logger) Debugf(msg string, args ...interface{}) {
	l.output("DEBUG", "blue", msg, args...)
}

// Info comment
func (l *Logger) Info(args ...interface{}) {
	msg := ""
	l.output("INFO", "green", msg, args...)
}

// Infof comment
func (l *Logger) Infof(msg string, args ...interface{}) {
	l.output("INFO", "green", msg, args...)
}

// Warn comment
func (l *Logger) Warn(args ...interface{}) {
	msg := ""
	l.output("WARN", "yellow", msg, args...)
}

// Warnf comment
func (l *Logger) Warnf(msg string, args ...interface{}) {
	l.output("WARN", "yellow", msg, args...)
}

// Error comment
func (l *Logger) Error(args ...interface{}) {
	msg := ""
	l.output("ERROR", "red", msg, args...)
}

// Errorf comment
func (l *Logger) Errorf(msg string, args ...interface{}) {
	l.output("ERROR", "red", msg, args...)
}

// ErrorWithStack comment
func (l *Logger) ErrorWithStack(msg string, args ...interface{}) {
	args = append(args, l.getStack())
	msg = msg + "\n%s"
	l.output("ERROR", "red", msg, args...)
}

// Fatal Comment
func (l *Logger) Fatal(args ...interface{}) {
	msg := ""
	l.output("FATAL", "cyan", msg, args...)
	if l.conf.socket != nil {
		l.conf.socket.Close()
	}
	trace := fmt.Sprintf("%s\n%s", args, debug.Stack())
	l.errorLog.Output(3, trace)
	// l.errorLog.Fatal(args...)
}

// Fatalf Comment
func (l *Logger) Fatalf(msg string, args ...interface{}) {
	l.output("FATAL", "cyan", msg, args...)
	if l.conf.socket != nil {
		l.conf.socket.Close()
	}
	l.errorLog.Fatal(fmt.Sprintf(msg, args...))
}

// Panic Comment
func (l *Logger) Panic(args ...interface{}) {
	msg := ""
	l.output("PANIC", "magenta", msg, args...)
	if l.conf.socket != nil {
		l.conf.socket.Close()
	}
	l.errorLog.Panic(args...)
}

// Panicf Comment
func (l *Logger) Panicf(msg string, args ...interface{}) {
	l.output("PANIC", "magenta", msg, args...)
	if l.conf.socket != nil {
		l.conf.socket.Close()
	}
	l.errorLog.Panic(fmt.Sprintf(msg, args...))
}

func (l *Logger) output(level, colour, msg string, args ...interface{}) {
	print := true

	var logger *log.Logger
	switch level {
	case "DEBUG":
		if !l.isDebugEnabled() || !utils.IsRegexMatch(l.conf.debug.regex, fmt.Sprintf(msg, args...)) {
			print = false
		}
		logger = l.errorLog
	case "ERROR":
		logger = l.errorLog
	default:
		logger = l.infoLog
	}

	if l.colour && colour != "" {
		level = ansi.Color(level, colour)
	}

	format := fmt.Sprintf("%s - %s:", l.packageName, level)
	if msg != "" {
		format = fmt.Sprintf("%s - %s: %s", l.packageName, level, msg)
	}

	if print {
		if msg != "" {
			logger.Printf(format, args...)
		} else {
			m := []interface{}{format}
			m = append(m, args...)
			logger.Println(m...)
		}
	}

	s := time.Now().UTC().Format("2006-01-02 15:04:05.000 ")

	s += fmt.Sprintf(format, args...)

	if strings.HasSuffix(s, "\n") == false {
		s += "\n"
	}

	l.sendToTrace(s, level)

	l.sendToSample(s, level)
}

func (l *Logger) handleShutdown(ln net.Listener, c chan os.Signal) {
	// Shut down the socket if the application closes
	go func() {
		<-c
		l.Infof("LOGGER: Shutting down unix socket for Logger")
		ln.Close()
		os.Exit(0)
	}()
}

func (l *Logger) getStack() string {
	return strings.Join(strings.Split(string(debug.Stack()), "\n")[7:], "\n")
}

func (l *Logger) isDebugEnabled() bool {
	l.conf.mu.RLock()
	defer l.conf.mu.RUnlock()
	return l.conf.debug.enabled
}

func (l *Logger) sendToTrace(s string, level string) {
	l.conf.mu.Lock()
	defer l.conf.mu.Unlock()

	for sock, r := range l.conf.trace.sockets {
		if utils.IsRegexMatch(r, s) || utils.IsRegexMatch(strings.ToLower(r), strings.ToLower(level)) {
			_, e := sock.Write([]byte(s))
			if e != nil {
				l.errorLog.Println(ansi.Color(fmt.Sprintf("Writing client error: '%s'", e), "red"))
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

func (l *Logger) handleIncomingMessage(c net.Conn) {
	l.welcome(c)
	go func() {
		scanner := bufio.NewScanner(c)
		for scanner.Scan() {
			cmd := scanner.Text()
			s, err := utils.SplitArgs(cmd)
			if err != nil {
				l.write(c, fmt.Sprintf("  Cannot split command: %v\n\n", err))
			}

			switch s[0] {
			case "config":
				l.handleConfig(s, c)
			case "debug":
				l.handleDebugAndTrace("DEBUG", s, c)
			case "trace":
				l.handleDebugAndTrace("TRACE", s, c)
			case "sample":
				l.handleSample(s, c)
			case "status":
				l.sendStatus(c)
			case "quit":
				c.Close()
				return
			case "help":
				l.help(c)
			case "":

			default:
				l.write(c, fmt.Sprintf("  Command not found: %s\n\n", cmd))
			}
		}
	}()
}

func (l *Logger) handleTrace(r []string, c net.Conn) {
	l.conf.mu.Lock()
	defer l.conf.mu.Unlock()

	if len(r) <= 1 {
		l.write(c, "  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
		return
	}

	if r[1] == "off" {
		delete(l.conf.trace.sockets, c)
	}

	reg := ""
	if len(r) == 3 {
		reg = r[2]
	}

	if r[1] == "on" {
		l.conf.trace.sockets[c] = reg
	}
}

func (l *Logger) handleSample(r []string, c net.Conn) {

	if len(r) <= 1 {
		l.write(c, "  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
		return
	}

	switch r[1] {
	case "list":
		if len(r) != 2 {
			l.write(c, "  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
			return
		}

		if len(l.conf.samplers) == 0 {
			err := l.write(c, "  No running samples.\n\n")
			if err != nil {
				return
			}
		} else {
			s := ""
			for _, sampler := range l.conf.samplers {
				s += fmt.Sprintf("  %s\n", sampler)
			}
			s += "\n"

			err := l.write(c, s)
			if err != nil {
				break
			}
		}

	case "start":
		if len(r) < 4 || len(r) > 5 {
			l.write(c, "  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
			return
		}

		id := r[2]

		filename := r[3]

		regex := ""
		if len(r) == 5 {
			regex = r[4]
		}

		sampler, err := sampler.New(id, filename, regex)
		if err != nil {
			l.write(c, fmt.Sprintf("  Could not create sampler [%v].\n\n", err))
			return
		}

		l.conf.mu.Lock()
		l.conf.samplers = append(l.conf.samplers, sampler)
		l.conf.mu.Unlock()

		l.sendStatus(c)

	case "stop":
		if len(r) != 3 {
			l.write(c, "  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
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

		l.sendStatus(c)
	default:
		l.write(c, "  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
		return
	}
	return
}

func (l *Logger) handleConfig(r []string, c net.Conn) {
	if len(r) <= 1 {
		l.write(c, "  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
		return
	}

	switch r[1] {
	case "show":
		stats := Config().Stats()
		l.write(c, stats+"\n\n")

	case "get":
		value, ok := Config().Get(r[2])
		if !ok {
			l.write(c, "  Not set\n\n")
		} else {
			l.write(c, fmt.Sprintf("  %s=%s\n\n", r[2], value))
		}

	case "set":
		oldValue := Config().Set(r[2], r[3])

		if oldValue == r[3] {
			l.write(c, "  No change\n\n")
		} else if oldValue == "" {
			l.write(c, fmt.Sprintf("  Created new setting: %s=%s\n\n", r[2], r[3]))
		} else {
			l.write(c, fmt.Sprintf("  Updated setting: %s %s -> %s\n\n", r[2], oldValue, r[3]))
		}
	case "unset":
		oldValue := Config().Unset(r[2])
		if oldValue == "" {
			l.write(c, "  No change\n\n")
		} else {
			l.write(c, fmt.Sprintf("  Removed setting: %s=%s\n\n", r[2], oldValue))
		}
	}
}

func (l *Logger) handleDebugAndTrace(context string, r []string, c net.Conn) {
	if len(r) <= 1 {
		l.write(c, "  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
		return
	}

	switch r[1] {
	case "off":
		if len(r) != 2 {
			l.write(c, "  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
			return
		}
		switch context {
		case "DEBUG":
			l.toggleDebug(false, "")
			l.sendStatus(c)
		case "TRACE":
			l.conf.mu.Lock()
			delete(l.conf.trace.sockets, c)
			l.conf.mu.Unlock()
			l.sendStatus(c)
		default:
			l.write(c, "Invalid context'\n")
		}

	case "on":
		if len(r) > 3 {
			l.write(c, "  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
			return
		}

		reg := ""
		if len(r) == 3 {
			reg = r[2]
		}

		switch context {
		case "DEBUG":
			l.toggleDebug(true, reg)
			l.sendStatus(c)
		case "TRACE":
			l.conf.mu.Lock()
			l.conf.trace.sockets[c] = reg
			l.conf.mu.Unlock()
			l.sendStatus(c)
		default:
			l.write(c, "Invalid context'\n")
		}

	default:
		l.write(c, "  Second parameter must be 'on' or 'off'\n\n")
	}
}

func (l *Logger) toggleDebug(enabled bool, regex string) {
	l.conf.mu.Lock()
	defer l.conf.mu.Unlock()

	l.conf.debug.enabled = enabled
	l.conf.debug.regex = regex

	return
}

func (l *Logger) sendStatus(c net.Conn) {
	l.conf.mu.RLock()
	defer l.conf.mu.RUnlock()

	res := ""
	if l.conf.debug.enabled {
		if l.conf.debug.regex != "" {
			res += fmt.Sprintf("  DEBUG is ON filtered by a regex of %q\n", l.conf.debug.regex)
		} else {
			res += "  DEBUG is ON with no filter\n"
		}
	} else {
		res += "  DEBUG is OFF\n"
	}

	if regex, ok := l.conf.trace.sockets[c]; ok {
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

	l.write(c, res)
}

func (l *Logger) help(c net.Conn) {
	type command struct {
		cmd         string
		description string
	}

	cmds := []command{
		command{
			cmd:         "debug [on {regex} | off] ",
			description: "Turn on/off debug mode with an optional Regex pattern",
		},
		command{
			cmd:         "trace [on {regex} | off] ",
			description: "Turn on/off trace mode with an optional Regex pattern",
		},
		command{
			cmd:         "sample [start <id> <filename> {regex} | stop <id> | list] ",
			description: "Turn on/off samplers mode with an optional Regex pattern",
		},
		command{
			cmd:         "config [get <key> | set <key> <value> | unset <key> | show ] ",
			description: "Manage settings dynamically",
		},
		command{
			cmd:         "status",
			description: "Shows the status of debug",
		},
		command{
			cmd:         "help",
			description: "Shows the available commands",
		},
		command{
			cmd:         "quit",
			description: "Terminates this session",
		},
	}

	res := ""
	for _, c := range cmds {
		res += fmt.Sprintf("  %s (%s)\n", c.cmd, c.description)
	}
	res += "\n"

	l.write(c, res)
}

func (l *Logger) welcome(c net.Conn) {

	res := "Runtime logger controller.\n-------------------------\nType help for a list of available commands.\n\n"
	l.write(c, res)
}
