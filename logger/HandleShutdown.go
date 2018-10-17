package logger

import (
	"log"
	"net"
	"os"
)

// HandleShutdown Comment
func (l *Logger) HandleShutdown(ln net.Listener, c chan os.Signal) {
	// Shut down the socket if the application closes
	go func() {
		<-c
		log.Printf("LOGGER: Shutting down unix socket for Logger")
		ln.Close()
		os.Exit(0)
	}()
}
