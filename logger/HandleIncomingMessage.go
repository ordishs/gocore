package logger

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
)

// HandleIncomingMessage Comment
func (l *Logger) HandleIncomingMessage(c net.Conn) {
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
					log.Printf("Writing client error: %+v", err)
				}
			}
		}
	}()
}

func (l *Logger) handleTrace(r []string, c net.Conn) {
	l.Conf.Mu.Lock()
	defer l.Conf.Mu.Unlock()

	if len(r) <= 1 {
		_, err := c.Write([]byte("Invalid number of parameters. Use 'help' to see the syntax.\n"))
		if err != nil {
			log.Printf("Writing client error: %+v", err)
		}
		return
	}

	if r[1] == "off" {
		delete(l.Conf.Trace.Sockets, c)
	}

	reg := ""
	if len(r) == 3 {
		reg = r[2]
	}

	if r[1] == "on" {
		l.Conf.Trace.Sockets[c] = reg
	}

}

func (l *Logger) handleDebug(r []string, c net.Conn) {
	l.Conf.Mu.Lock()
	defer l.Conf.Mu.Unlock()

	if len(r) <= 1 {
		_, err := c.Write([]byte("Invalid number of parameters. Use 'help' to see the syntax.\n"))
		if err != nil {
			log.Printf("Writing client error: %+v", err)
		}
		return
	}

	if r[1] != "off" && r[1] != "on" {
		_, err := c.Write([]byte("Second parameter must be 'on' or 'off'\n"))
		if err != nil {
			log.Printf("Writing client error: %+v", err)
		}
		return
	}

	if r[1] == "off" {
		l.Conf.Debug.Enabled = false
		l.Conf.Debug.Regex = ""
		l.sendStatus(c)
		return
	}

	reg := ""
	if len(r) == 3 {
		reg = r[2]
	}

	l.Conf.Debug.Enabled = true
	l.Conf.Debug.Regex = reg

	l.sendStatus(c)

}

func (l *Logger) sendStatus(c net.Conn) {
	l.Conf.Mu.RLock()
	defer l.Conf.Mu.RUnlock()

	res := fmt.Sprintf(
		"Debug status is set to %t, with a regex of '%s'\n",
		l.Conf.Debug.Enabled, l.Conf.Debug.Regex,
	)
	_, err := c.Write([]byte(res))
	if err != nil {
		log.Printf("Writing client error: %+v", err)
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
	}

	res := "Available Commands:\n"
	for _, c := range cmds {
		res += fmt.Sprintf("\t\t-- %s (%s)\n", c.cmd, c.description)
	}

	_, err := c.Write([]byte(res))
	if err != nil {
		log.Printf("Writing client error: %+v", err)
	}
}
