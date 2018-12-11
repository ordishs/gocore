package gocore

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/mgutz/ansi"
)

var socketDIR string

func init() {
	socketDIR, _ = Config().Get("socketDIR")
	if socketDIR == "" {
		socketDIR = "/tmp/maestro"
	}
	err := os.MkdirAll(socketDIR, os.ModePerm)
	if err != nil {
		log.Printf("ERROR: Unable to make socket directory %s: %+v", socketDIR, err)
	}
}

var logger *Logger

// Log comment
func Log(packageName string) *Logger {
	if logger == nil {
		logger = &Logger{
			packageName: packageName,
			colour:      true,
			conf: loggerConfig{
				mu: new(sync.RWMutex),
				trace: traceSettings{
					sockets: make(map[net.Conn]string),
				},
			},
		}

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
	}

	return logger
}

// Debugf Comment
func (l *Logger) Debugf(msg string, args ...interface{}) {
	l.output("DEBUG", "blue", msg, args...)
}

// Infof comment
func (l *Logger) Infof(msg string, args ...interface{}) {
	l.output("INFO", "green", msg, args...)
}

// Warnf comment
func (l *Logger) Warnf(msg string, args ...interface{}) {
	l.output("WARN", "yellow", msg, args...)
}

// Errorf comment
func (l *Logger) Errorf(msg string, args ...interface{}) {
	args = append(args, l.getStack())
	msg = msg + "\n%s"
	l.output("ERROR", "red", msg, args...)
}

// Fatal Comment
func (l *Logger) Fatal(args ...interface{}) {
	l.output("FATAL", "cyan", "%s", args...)
	if l.conf.socket != nil {
		l.conf.socket.Close()
	}
	log.Fatal(args...)
}

// Fatalf Comment
func (l *Logger) Fatalf(msg string, args ...interface{}) {
	l.output("FATAL", "cyan", msg, args...)
	if l.conf.socket != nil {
		l.conf.socket.Close()
	}
	log.Fatal(args...)
}

// Panic Comment
func (l *Logger) Panic(args ...interface{}) {
	l.output("PANIC", "magenta", "%s", args...)
	if l.conf.socket != nil {
		l.conf.socket.Close()
	}
	log.Panic(args...)
}

// Panicf Comment
func (l *Logger) Panicf(msg string, args ...interface{}) {
	l.output("PANIC", "magenta", msg, args...)
	if l.conf.socket != nil {
		l.conf.socket.Close()
	}
	log.Panic(args...)
}

func (l *Logger) output(level, colour, msg string, args ...interface{}) {
	print := true

	if level == "DEBUG" {
		if !l.isDebugEnabled() || !l.isRegexMatch(l.conf.debug.regex, fmt.Sprintf(msg, args...)) {
			print = false
		}
	}

	if l.colour && colour != "" {
		level = ansi.Color(level, colour)
	}

	format := fmt.Sprintf("%s - %s: %s\n", l.packageName, level, msg)

	if print {
		log.Printf(format, args...)
	}

	l.sendToTrace(format, msg, level, args...)
}
