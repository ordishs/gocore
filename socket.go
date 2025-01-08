package gocore

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"

	"github.com/ordishs/gocore/utils"
)

// SocketHandler handles incoming socket connections
type SocketHandler struct {
	logger *Logger
	rwc    io.ReadWriteCloser
}

var (
	socketDIR string
)

// NewSocketHandler creates a new socket handler
func NewSocketHandler(logger *Logger, rwc io.ReadWriteCloser) *SocketHandler {
	return &SocketHandler{
		logger: logger,
		rwc:    rwc,
	}
}

// Handle processes incoming messages on the socket
func (h *SocketHandler) Handle() {
	h.welcome()

	scanner := bufio.NewScanner(h.rwc)
	prompt := fmt.Sprintf("gocore::%s> ", h.logger.packageName)

	for {
		_ = h.write(prompt)
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		s, err := utils.SplitArgs(line)
		if err != nil {
			if err := h.write(fmt.Sprintf("  Cannot split command: %v\n\n", err)); err != nil {
				return
			}
		}

		switch s[0] {
		case "config":
			h.handleConfig(s)
		case "trace":
			h.handleDebugAndTrace("TRACE", s)
		case "loglevel":
			h.handleLogLevel(s)
		case "sample":
			h.handleSample(s)
		case "status":
			h.sendStatus()
		case "quit":
			fallthrough
		case "exit":
			_ = h.rwc.Close()
			return
		case "help":
			h.help()
		case "":

		default:
			if err := h.write(fmt.Sprintf("  Command not found: %s\n\n", line)); err != nil {
				return
			}
		}
	}
}

// write sends a message to the socket
func (h *SocketHandler) write(s string) error {
	_, err := h.rwc.Write([]byte(s))
	return err
}

// welcome sends the welcome message
func (h *SocketHandler) welcome() {
	_ = h.write(fmt.Sprintf("\nWelcome to gocore for %s\n\n", h.logger.packageName))
}

// handleConfig processes config commands
func (h *SocketHandler) handleConfig(r []string) {
	defer func() {
		if r := recover(); r != nil {
			_ = h.write(fmt.Sprintf("  ERROR: Recovered from panic in handleConfig: %v\n\n", r))
		}
	}()

	if len(r) <= 1 {
		_ = h.write("  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
		return
	}

	switch r[1] {
	case "requested":
		requested := Config().Requested()
		_ = h.write(fmt.Sprintf("\n%s\n\n", requested))

	case "show":
		stats := Config().Stats()
		_ = h.write(stats + "\n\n")

	case "get":
		if len(r) < 3 {
			_ = h.write("  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
			return
		}
		value, ok := Config().Get(r[2])
		if !ok {
			_ = h.write("  Not set\n\n")
		} else {
			_ = h.write(fmt.Sprintf("  %s=%s\n\n", r[2], value))
		}

	case "set":
		if len(r) < 3 {
			_ = h.write("  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
			return
		}

		var key, value string
		if len(r) >= 4 {
			// Traditional format: set key value
			key = r[2]
			value = strings.Join(r[3:], " ")
		} else if strings.Contains(r[2], "=") {
			// k=v format
			parts := strings.SplitN(strings.TrimSpace(r[2]), "=", 2)
			if len(parts) != 2 {
				_ = h.write("  Invalid format. Use either 'set key value' or 'set key=value'\n\n")
				return
			}
			key = strings.TrimSpace(parts[0])
			value = strings.TrimSpace(parts[1])
		} else {
			_ = h.write("  Invalid format. Use either 'set key value' or 'set key=value'\n\n")
			return
		}

		if key == "" {
			_ = h.write("  Key cannot be empty\n\n")
			return
		}

		oldValue := Config().Set(key, value)
		if oldValue == value {
			_ = h.write("  No change\n\n")
		} else if oldValue == "" {
			_ = h.write(fmt.Sprintf("  Created new setting: %s=%s\n\n", key, value))
		} else {
			_ = h.write(fmt.Sprintf("  Updated setting: %s %s -> %s\n\n", key, oldValue, value))
		}

	case "unset":
		if len(r) < 3 {
			_ = h.write("  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
			return
		}
		oldValue := Config().Unset(r[2])
		if oldValue == "" {
			_ = h.write("  No change\n\n")
		} else {
			_ = h.write(fmt.Sprintf("  Removed setting: %s=%s\n\n", r[2], oldValue))
		}

	default:
		_ = h.write("  Invalid command. Use 'help' to see the syntax.\n\n")
	}
}

// handleLogLevel processes log level commands
func (h *SocketHandler) handleLogLevel(r []string) {
	if len(r) <= 1 {
		_ = h.write("  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
		return
	}

	h.logger.setLogLevel(NewLogLevelFromString(r[1]))
	_ = h.write(fmt.Sprintf("  Log level set to %s\n\n", r[1]))
}

// handleDebugAndTrace processes debug and trace commands
func (h *SocketHandler) handleDebugAndTrace(context string, r []string) {
	if len(r) <= 1 {
		_ = h.write("  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
		return
	}

	switch r[1] {
	case "on":
		h.logger.conf.mu.Lock()
		if _, found := h.logger.conf.trace.sockets[h.rwc]; !found {
			h.logger.conf.trace.sockets[h.rwc] = ""
		}
		h.logger.conf.mu.Unlock()

		_ = h.write(fmt.Sprintf("  %s ON\n\n", context))

	case "off":
		h.logger.conf.mu.Lock()
		delete(h.logger.conf.trace.sockets, h.rwc)
		h.logger.conf.mu.Unlock()

		_ = h.write(fmt.Sprintf("  %s OFF\n\n", context))

	case "clear":
		h.logger.conf.mu.Lock()
		h.logger.conf.trace.sockets = make(map[io.ReadWriteCloser]string)
		h.logger.conf.mu.Unlock()

		_ = h.write(fmt.Sprintf("  %s CLEARED\n\n", context))

	default:
		_ = h.write("  Invalid parameter. Use 'help' to see the syntax.\n\n")
	}
}

// handleSample processes sample commands
func (h *SocketHandler) handleSample(r []string) {
	if len(r) <= 1 {
		_ = h.write("  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
		return
	}

	switch r[1] {
	case "on":
		if len(r) < 3 {
			_ = h.write("  Invalid number of parameters. Use 'help' to see the syntax.\n\n")
			return
		}

		h.logger.conf.mu.Lock()
		if _, found := h.logger.conf.trace.sockets[h.rwc]; !found {
			h.logger.conf.trace.sockets[h.rwc] = r[2]
		}
		h.logger.conf.mu.Unlock()

		_ = h.write("  SAMPLE ON\n\n")

	case "off":
		h.logger.conf.mu.Lock()
		delete(h.logger.conf.trace.sockets, h.rwc)
		h.logger.conf.mu.Unlock()

		_ = h.write("  SAMPLE OFF\n\n")

	case "clear":
		h.logger.conf.mu.Lock()
		h.logger.conf.trace.sockets = make(map[io.ReadWriteCloser]string)
		h.logger.conf.mu.Unlock()

		_ = h.write("  SAMPLE CLEARED\n\n")

	default:
		_ = h.write("  Invalid parameter. Use 'help' to see the syntax.\n\n")
	}
}

// sendStatus sends the current status
func (h *SocketHandler) sendStatus() {
	_ = h.write("\nStatus:\n")
	_ = h.write(fmt.Sprintf("  Log level: %s\n", h.logger.conf.logLevel.String()))
	_ = h.write(fmt.Sprintf("  Package: %s\n", h.logger.packageName))
	_ = h.write(fmt.Sprintf("  Colour: %v\n", h.logger.colour))
	_ = h.write(fmt.Sprintf("  Show timestamps: %v\n", h.logger.showTimestamp))
	_ = h.write("\n")
}

// help sends the help message
func (h *SocketHandler) help() {
	_ = h.write(`
Commands:
  config
    show                    Show all configuration settings
    requested              Show all requested configuration settings
    get <key>              Get a configuration setting
    set <key> <value>      Set a configuration setting
    unset <key>            Remove a configuration setting

  loglevel <level>         Set the log level (DEBUG, INFO, WARN, ERROR, FATAL)

  trace
    on                     Turn on tracing for this connection
    off                    Turn off tracing for this connection
    clear                  Clear all trace connections

  sample
    on <regex>             Turn on sampling for this connection with regex filter
    off                    Turn off sampling for this connection
    clear                  Clear all sample connections

  status                   Show current status

  help                     Show this help message

  quit                     Close the connection

`)
}

// StartSocketListener starts the Unix socket listener for a logger
func StartSocketListener(logger *Logger) error {
	socketDIR, _ = Config().Get("socketDIR")
	if socketDIR == "" {
		socketDIR = "/tmp/gocore"
	}

	err := os.MkdirAll(socketDIR, os.ModePerm)
	if err != nil {
		log.Printf("ERROR: Unable to make socket directory %s: %+v", socketDIR, err)
	}

	socketPath := fmt.Sprintf("%s/%s.sock", socketDIR, strings.ToUpper(logger.packageName))

	// Remove the file if it exists
	os.Remove(socketPath)

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("socket listen error: %v", err)
	}

	// Add the socket so we can close it down when Fatal or Panic are called
	logger.conf.socket = ln

	if Config().GetBool("logger_show_socket_info", true) {
		logger.Infof("Socket created. Connect with: nc -U %s", socketPath)
	}

	go func() {
		defer ln.Close()
		defer os.Remove(socketPath)

		for {
			conn, err := ln.Accept()
			if err != nil {
				if Config().GetBool("logger_show_socket_info", false) {
					logger.Warnf("Accept error: %v", err)
				}
				return
			}

			handler := NewSocketHandler(logger, conn)
			go handler.Handle()
		}
	}()

	return nil
}
