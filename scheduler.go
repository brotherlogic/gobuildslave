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
	endTime   int64
	err       error
}

//Scheduler the main task scheduler
type Scheduler struct {
	commands []*rCommand
	cMutex   *sync.Mutex
	rMutex   *sync.Mutex
	rMap     map[string]*rCommand
}

func (s *Scheduler) markComplete(key string) {
	s.rMutex.Lock()
	if val, ok := s.rMap[key]; ok {
		val.endTime = time.Now().Unix()
	} else {
		s.rMap[key] = &rCommand{endTime: time.Now().Unix()}
	}
	s.rMutex.Unlock()
}

// Schedule schedules a task
func (s *Scheduler) Schedule(c *rCommand) string {
	key := fmt.Sprintf("%v-%v", time.Now().Nanosecond(), c.command.Path)
	s.commands = append(s.commands, c)
	s.rMutex.Lock()
	s.rMap[key] = c
	s.rMutex.Unlock()
	s.processCommands()
	return key
}

func (s *Scheduler) getOutput(key string) string {
	s.rMutex.Lock()
	if val, ok := s.rMap[key]; ok {
		s.rMutex.Unlock()
		return val.output
	}

	s.rMutex.Unlock()
	return ""
}

func (s *Scheduler) schedulerComplete(key string) bool {
	s.rMutex.Lock()
	if val, ok := s.rMap[key]; ok {
		s.rMutex.Unlock()
		return val.endTime > 0
	}

	s.rMutex.Unlock()
	return false
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

	gpath := home + "/gobuild"
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

	// Monitor the job and report completion
	go func() {
		err := c.command.Wait()
		c.endTime = time.Now().Unix()
		if err != nil {
			c.err = err
		}
	}()

	return nil
}
