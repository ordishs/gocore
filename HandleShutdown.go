package gocore

import (
	"net"
	"os"
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
