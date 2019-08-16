package gocore

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"net"
	"strings"

	"github.com/ordishs/gocore/sampler"
)

func splitArgs(s string) (args []string, err error) {
	if s == "" {
		return []string{""}, nil
	}
	r := csv.NewReader(strings.NewReader(s))
	r.Comma = ' ' // space
	args, err = r.Read()
	return
}

func (l *Logger) handleIncomingMessage(c net.Conn) {
	l.welcome(c)
	go func() {
		scanner := bufio.NewScanner(c)
		for scanner.Scan() {
			cmd := scanner.Text()
			s, err := splitArgs(cmd)
			if err != nil {
				_, err := c.Write([]byte(fmt.Sprintf("  Cannot split command: %v\n\n", err)))
				if err != nil {
					l.Errorf("Writing client error: %+v", err)
				}
			}

			switch s[0] {
			case "debug":
				l.handleDebugAndTrace("DEBUG", s, c)
			case "trace":
				l.handleDebugAndTrace("TRACE", s, c)
			case "sample":
				l.handleSample(s, c)
			case "status":
				l.sendStatus(c)
			case "quit":
				c.Close()
				return
			case "help":
				l.help(c)
			case "":

			default:
				_, err := c.Write([]byte(fmt.Sprintf("  Command not found: %s\n\n", cmd)))
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
		_, err := c.Write([]byte("  Invalid number of parameters. Use 'help' to see the syntax.\n\n"))
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

func (l *Logger) handleSample(r []string, c net.Conn) {

	if len(r) <= 1 {
		_, err := c.Write([]byte("  Invalid number of parameters. Use 'help' to see the syntax.\n\n"))
		if err != nil {
			l.Errorf("Writing client error: %+v", err)
		}
		return
	}

	switch r[1] {
	case "list":
		if len(r) != 2 {
			_, err := c.Write([]byte("  Invalid number of parameters. Use 'help' to see the syntax.\n\n"))
			if err != nil {
				l.Errorf("Writing client error: %+v", err)
			}
			return
		}

		if len(l.conf.samplers) == 0 {
			_, err := c.Write([]byte("  No running samples.\n\n"))
			if err != nil {
				l.Errorf("Writing client error: %+v", err)
				return
			}
		} else {
			for i, j := range l.conf.samplers {
				_, err := c.Write([]byte(fmt.Sprintf("  Samples %v %v.\n", i, j)))
				if err != nil {
					l.Errorf("Writing client error: %+v", err)
					break
				}
			}
		}
	case "on":
		if len(r) < 4 || len(r) > 5 {
			_, err := c.Write([]byte("  Invalid number of parameters. Use 'help' to see the syntax.\n\n"))
			if err != nil {
				l.Errorf("Writing client error: %+v", err)
			}
			return
		}

		id := r[2]

		filename := r[3]

		regex := ""
		if len(r) == 5 {
			regex = r[4]
		}

		sampler, err := sampler.New(id, filename, regex)
		if err != nil {
			_, err := c.Write([]byte(fmt.Sprintf("  Could not create sampler [%v].\n\n", err)))
			if err != nil {
				l.Errorf("Writing client error: %+v", err)
			}
			return
		}

		l.conf.mu.Lock()
		l.conf.samplers = append(l.conf.samplers, sampler)
		l.conf.mu.Unlock()

		l.sendStatus(c)
	case "off":
		if len(r) != 3 {
			_, err := c.Write([]byte("  Invalid number of parameters. Use 'help' to see the syntax.\n\n"))
			if err != nil {
				l.Errorf("Writing client error: %+v", err)
			}
			return
		}

		id := r[2]

		l.conf.mu.Lock()
		for i, sampler := range l.conf.samplers {
			if sampler.ID == id {
				sampler.Stop()
				l.conf.samplers = append(l.conf.samplers[:i], l.conf.samplers[i+1:]...)
				break // Only close the first sampler with this ID in case there are more than one with the same ID
			}
		}
		l.conf.mu.Unlock()

		l.sendStatus(c)
	default:
		_, err := c.Write([]byte("  Invalid number of parameters. Use 'help' to see the syntax.\n\n"))
		if err != nil {
			l.Errorf("Writing client error: %+v", err)
		}
		return
	}
	return
}

func (l *Logger) handleDebugAndTrace(context string, r []string, c net.Conn) {
	if len(r) <= 1 {
		_, err := c.Write([]byte("  Invalid number of parameters. Use 'help' to see the syntax.\n\n"))
		if err != nil {
			l.Errorf("Writing client error: %+v", err)
		}
		return
	}

	switch r[1] {
	case "off":
		if len(r) != 2 {
			_, err := c.Write([]byte("  Invalid number of parameters. Use 'help' to see the syntax.\n\n"))
			if err != nil {
				l.Errorf("Writing client error: %+v", err)
			}
			return
		}
		switch context {
		case "DEBUG":
			l.toggleDebug(false, "")
			l.sendStatus(c)
		case "TRACE":
			delete(l.conf.trace.sockets, c)
			l.sendStatus(c)
		default:
			_, err := c.Write([]byte("Invalid context'\n"))
			if err != nil {
				l.Errorf("Writing client error: %+v", err)
			}
		}

	case "on":
		if len(r) > 3 {
			_, err := c.Write([]byte("  Invalid number of parameters. Use 'help' to see the syntax.\n\n"))
			if err != nil {
				l.Errorf("Writing client error: %+v", err)
			}
			return
		}

		reg := ""
		if len(r) == 3 {
			reg = r[2]
		}

		switch context {
		case "DEBUG":
			l.toggleDebug(true, reg)
			l.sendStatus(c)
		case "TRACE":
			l.conf.trace.sockets[c] = reg
			l.sendStatus(c)
		default:
			_, err := c.Write([]byte("Invalid context'\n"))
			if err != nil {
				l.Errorf("Writing client error: %+v", err)
			}
		}

	default:
		_, err := c.Write([]byte("  Second parameter must be 'on' or 'off'\n\n"))
		if err != nil {
			l.Errorf("Writing client error: %+v", err)
		}
	}
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

	res := ""
	if l.conf.debug.enabled {
		if l.conf.debug.regex != "" {
			res += fmt.Sprintf("  DEBUG is ON filtered by a regex of %q\n", l.conf.debug.regex)
		} else {
			res += "  DEBUG is ON with no filter\n"
		}
	} else {
		res += "  DEBUG is OFF\n"
	}

	if regex, ok := l.conf.trace.sockets[c]; ok {
		if regex != "" {
			res += fmt.Sprintf("  TRACE is ON filtered by a regex of %q\n", regex)
		} else {
			res += "  TRACE is ON with no filter\n"
		}
	} else {
		res += "  TRACE is OFF\n"
	}

	res += fmt.Sprintf("  %d SAMPLES configured\n", len(l.conf.samplers))

	res += "\n"

	_, err := c.Write([]byte(res))
	if err != nil {
		l.Errorf("Writing client error: %+v", err)
	}
}

func (l *Logger) help(c net.Conn) {
	type command struct {
		cmd         string
		description string
	}

	cmds := []command{
		command{
			cmd:         "debug [on {regex} | off] ",
			description: "Turn on/off debug mode with an optional Regex pattern",
		},
		command{
			cmd:         "trace [on {regex} | off] ",
			description: "Turn on/off trace mode with an optional Regex pattern",
		},
		command{
			cmd:         "sample [on <id> <filename> {regex} | off <id> | list] ",
			description: "Turn on/off samplers mode with an optional Regex pattern",
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
			description: "Terminates this session",
		},
	}

	res := ""
	for _, c := range cmds {
		res += fmt.Sprintf("  %s (%s)\n", c.cmd, c.description)
	}
	res += "\n"

	_, err := c.Write([]byte(res))
	if err != nil {
		l.Errorf("Writing client error: %+v", err)
	}
}

func (l *Logger) welcome(c net.Conn) {

	res := "Runtime logger controller.\n-------------------------\nType help for a list of available commands.\n\n"
	_, err := c.Write([]byte(res))
	if err != nil {
		l.Errorf("Writing client error: %+v", err)
	}
}
