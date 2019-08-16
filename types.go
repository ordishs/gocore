package gocore

import (
	"net"
	"sync"

	"github.com/ordishs/gocore/sampler"
)

// DebugSettings Comment
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
}
