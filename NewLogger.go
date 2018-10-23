package gocore

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
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

// NewLogger comment
func NewLogger(packageName string, serviceName string, enableColours bool) *Logger {
	logger := Logger{
		packageName: strings.ToUpper(packageName),
		serviceName: strings.ToUpper(serviceName),
		colour:      enableColours,
		conf: loggerConfig{
			mu: new(sync.RWMutex),
			trace: traceSettings{
				sockets: make(map[net.Conn]string),
			},
		},
	}

	// Run a listener on a Unix socket
	go func() {
		n := fmt.Sprintf("%s/%s.%s%d.sock", socketDIR, strings.ToUpper(packageName), strings.ToUpper(serviceName), getRand())

		ln, err := net.Listen("unix", n)
		if err != nil {
			logger.Fatalf("LOGGER: listen error: %+v", err)
		}
		// Add the socket so we can close it down when Fatal or Panic are called
		logger.conf.socket = ln

		logger.Infof("Socket created. Connect with 'nc -U %s'", n)

		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

		logger.handleShutdown(ln, ch)

		for {
			fd, err := ln.Accept()
			if err != nil {
				logger.Fatalf("Accept error: %+v", err)
			}

			logger.handleIncomingMessage(fd)
		}

	}()

	return &logger
}

func getRand() int {
	rand.Seed(time.Now().UnixNano())
	min := 100000000
	max := 999999999
	return rand.Intn(max-min) + min
}
