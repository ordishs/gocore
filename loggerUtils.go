package gocore

import (
	"fmt"
	"log"
	"net"
	"os"
	"regexp"
	"runtime/debug"
	"strings"

	"github.com/mgutz/ansi"
)

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

func (l *Logger) sendToTrace(format string, msg string, args ...interface{}) {
	l.conf.mu.Lock()
	defer l.conf.mu.Unlock()

	for s := range l.conf.trace.sockets {
		if l.isRegexMatch(msg, args...) {
			_, e := s.Write([]byte(fmt.Sprintf(format, args...)))
			if e != nil {
				log.Println(ansi.Color(fmt.Sprintf("Writing client error: '%s'", e), "red"))
				delete(l.conf.trace.sockets, s)
			}
		}
	}
}

func (l *Logger) isRegexMatch(msg string, args ...interface{}) bool {
	match, _ := regexp.MatchString(l.conf.debug.regex, fmt.Sprintf(msg, args...))
	return match
}
