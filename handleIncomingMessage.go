package gocore

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

func (l *Logger) handleIncomingMessage(c net.Conn) {
	go func() {
		scanner := bufio.NewScanner(c)
		for scanner.Scan() {
			cmd := scanner.Text()
			s := strings.Split(cmd, " ")
			switch s[0] {
			case "debug":
				l.handleDebug(s, c)
			case "trace":
				l.handleTrace(s, c)
			case "status":
				l.sendStatus(c)
			case "quit":
				c.Close()
				return
			case "help":
				l.getCommands(c)
			case "":

			default:
				_, err := c.Write([]byte(fmt.Sprintf("Command not found: %s\n", cmd)))
				if err != nil {
					l.Errorf("Writing client error: %+v", err)
				}
			}
		}
	}()
}

func (l *Logger) handleTrace(r []string, c net.Conn) {
	l.conf.mu.Lock()
	defer l.conf.mu.Unlock()

	if len(r) <= 1 {
		_, err := c.Write([]byte("Invalid number of parameters. Use 'help' to see the syntax.\n"))
		if err != nil {
			l.Errorf("Writing client error: %+v", err)
		}
		return
	}

	if r[1] == "off" {
		delete(l.conf.trace.sockets, c)
	}

	reg := ""
	if len(r) == 3 {
		reg = r[2]
	}

	if r[1] == "on" {
		l.conf.trace.sockets[c] = reg
	}
}

func (l *Logger) handleDebug(r []string, c net.Conn) {
	if len(r) <= 1 {
		_, err := c.Write([]byte("Invalid number of parameters. Use 'help' to see the syntax.\n"))
		if err != nil {
			l.Errorf("Writing client error: %+v", err)
		}
		return
	}

	if r[1] != "off" && r[1] != "on" {
		_, err := c.Write([]byte("Second parameter must be 'on' or 'off'\n"))
		if err != nil {
			l.Errorf("Writing client error: %+v", err)
		}
		return
	}

	if r[1] == "off" {
		l.toggleDebug(false, "")
		l.sendStatus(c)
		return
	}

	reg := ""
	if len(r) == 3 {
		reg = r[2]
	}

	l.toggleDebug(true, reg)
	l.sendStatus(c)

}

func (l *Logger) toggleDebug(enabled bool, regex string) {
	l.conf.mu.Lock()
	defer l.conf.mu.Unlock()

	l.conf.debug.enabled = enabled
	l.conf.debug.regex = regex

	return
}

func (l *Logger) sendStatus(c net.Conn) {
	l.conf.mu.RLock()
	defer l.conf.mu.RUnlock()

	res := fmt.Sprintf(
		"Debug status is set to %t, with a regex of '%s'\n",
		l.conf.debug.enabled, l.conf.debug.regex,
	)
	_, err := c.Write([]byte(res))
	if err != nil {
		l.Errorf("Writing client error: %+v", err)
	}
}

func (l *Logger) getCommands(c net.Conn) {
	type command struct {
		cmd         string
		description string
	}

	cmds := []command{
		command{
			cmd:         "debug {on/off} {regex}",
			description: "Turn on/off debug mode with an optional Regex pattern",
		},
		command{
			cmd:         "trace {on/off} {regex}",
			description: "Turn on/off trace mode with an optional Regex pattern",
		},
		command{
			cmd:         "status",
			description: "Shows the status of debug",
		},
		command{
			cmd:         "help",
			description: "Shows the available commands",
		},
		command{
			cmd:         "quit",
			description: "quit the socket window",
		},
	}

	res := "Available Commands:\n"
	for _, c := range cmds {
		res += fmt.Sprintf("\t\t-- %s (%s)\n", c.cmd, c.description)
	}

	_, err := c.Write([]byte(res))
	if err != nil {
		l.Errorf("Writing client error: %+v", err)
	}
}
