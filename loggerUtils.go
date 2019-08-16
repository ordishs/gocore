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

func (l *Logger) sendToTrace(s string, level string) {
	l.conf.mu.Lock()
	defer l.conf.mu.Unlock()

	for sock, r := range l.conf.trace.sockets {
		if l.isRegexMatch(r, s) || l.isRegexMatch(strings.ToLower(r), strings.ToLower(level)) {
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
		if l.isRegexMatch(sampler.Regex, s) || l.isRegexMatch(strings.ToLower(sampler.Regex), strings.ToLower(level)) {
			sampler.Write(s)
		}
	}
}

func (l *Logger) isRegexMatch(r string, msg string) bool {
	match, _ := regexp.MatchString(r, msg)
	return match
}
