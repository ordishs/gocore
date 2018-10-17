package logger

import (
	"net"
	"sync"
)

// DebugSettings Comment
type DebugSettings struct {
	Enabled bool
	Regex   string
}

// TraceSettings Comment
type TraceSettings struct {
	Enabled bool
	// String will be a Regex expression
	Sockets map[net.Conn]string
}

// Config comment
type Config struct {
	Mu    *sync.RWMutex
	Debug DebugSettings
	Trace TraceSettings
}

// Logger comment
type Logger struct {
	PackageName string
	ServiceName string
	Colour      bool
	Conf        Config
}
