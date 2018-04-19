package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type rCommand struct {
	command   *exec.Cmd
	output    string
	startTime int64
}

//Scheduler the main task scheduler
type Scheduler struct {
	commands []*rCommand
	cMutex   *sync.Mutex
}

// Schedule schedules a task
func (s *Scheduler) Schedule(c *rCommand) {
	s.commands = append(s.commands, c)
	s.processCommands()
}

func (s *Scheduler) processCommands() {
	s.cMutex.Lock()
	if len(s.commands) > 0 {
		c := s.commands[0]
		s.commands = s.commands[1:]
		run(c)
	}
	s.cMutex.Unlock()
}

func run(c *rCommand) error {
	env := os.Environ()
	home := ""
	for _, s := range env {
		if strings.HasPrefix(s, "HOME=") {
			home = s[5:]
		}
	}

	gpath := home + "/gbuild"
	c.command.Path = strings.Replace(c.command.Path, "$GOPATH", gpath, -1)
	for i := range c.command.Args {
		c.command.Args[i] = strings.Replace(c.command.Args[i], "$GOPATH", gpath, -1)
	}
	path := fmt.Sprintf("GOPATH=" + home + "/gobuild")
	pathbin := fmt.Sprintf("GOBIN=" + home + "/gobuild/bin")
	found := false
	for i, blah := range env {
		if strings.HasPrefix(blah, "GOPATH") {
			env[i] = path
			found = true
		}
		if strings.HasPrefix(blah, "GOBIN") {
			env[i] = pathbin
			found = true
		}
	}
	if !found {
		env = append(env, path)
	}
	c.command.Env = env

	out, _ := c.command.StderrPipe()

	if out != nil {
		scanner := bufio.NewScanner(out)
		go func() {
			for scanner != nil && scanner.Scan() {
				c.output += scanner.Text()
			}
			out.Close()
		}()
	}

	err := c.command.Start()
	if err != nil {
		return err
	}
	c.startTime = time.Now().Unix()

	return nil
}
