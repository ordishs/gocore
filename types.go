package gocore

import (
	"net"
	"sync"
)

// DebugSettings Comment
type debugSettings struct {
	enabled bool
	regex   string
}

// TraceSettings Comment
type traceSettings struct {
	enabled bool
	// String will be a Regex expression
	sockets map[net.Conn]string
}

// LoggerConfig comment
type loggerConfig struct {
	mu     *sync.RWMutex
	socket net.Listener
	debug  debugSettings
	trace  traceSettings
}

// Logger comment
type Logger struct {
	packageName string
	colour      bool
	conf        loggerConfig
}
