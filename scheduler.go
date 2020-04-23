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
	mainOut   string
	status    string
	crash1    bool
	crash2    bool
	base      string
}

//Scheduler the main task scheduler
type Scheduler struct {
	commands []*rCommand
	cMutex   *sync.Mutex
	rMutex   *sync.Mutex
	rMap     map[string]*rCommand
	Log      func(string)
	lastRun  string
}

func (s *Scheduler) clean() {
	s.rMutex.Lock()
	defer s.rMutex.Unlock()
	for key, command := range s.rMap {
		if command.endTime > 0 && time.Now().Sub(time.Unix(command.endTime, 0)) > time.Minute*5 {
			delete(s.rMap, key)
		}
	}
}

func (s *Scheduler) getState(key string) string {
	s.rMutex.Lock()
	defer s.rMutex.Unlock()
	if _, ok := s.rMap[key]; ok {
		return fmt.Sprintf("%v -> %v", s.rMap[key].endTime, s.rMap[key].output)
	}

	return "UNKNOWN"
}

func (s *Scheduler) markComplete(key string) {
	s.rMutex.Lock()
	defer s.rMutex.Unlock()
	if val, ok := s.rMap[key]; ok {
		val.endTime = time.Now().Unix()
		val.output = key
	} else {
		s.rMap[key] = &rCommand{endTime: time.Now().Unix(), output: key, base: key}
	}
}

// Schedule schedules a task
func (s *Scheduler) Schedule(c *rCommand) string {
	key := fmt.Sprintf("%v-%v", time.Now().Nanosecond(), c.command.Path)
	s.commands = append(s.commands, c)
	c.status = "InQueue"
	s.rMutex.Lock()
	s.rMap[key] = c
	s.rMutex.Unlock()
	s.processCommands()
	return key
}

func (s *Scheduler) getOutput(key string) (string, error) {
	s.rMutex.Lock()
	defer s.rMutex.Unlock()
	if val, ok := s.rMap[key]; ok {
		return val.output, nil
	}

	return "", fmt.Errorf("KEY NOT_IN_MAP: %v", key)
}

func (s *Scheduler) getErrOutput(key string) (string, error) {
	s.rMutex.Lock()
	defer s.rMutex.Unlock()
	if val, ok := s.rMap[key]; ok {
		return val.mainOut, nil
	}

	return "", fmt.Errorf("KEY NOT_IN_MAP: %v", key)
}

func (s *Scheduler) getStatus(key string) string {
	s.rMutex.Lock()
	defer s.rMutex.Unlock()
	if val, ok := s.rMap[key]; ok {
		return val.status
	}

	return fmt.Sprintf("KEY NOT_IN_MAP: %v", key)
}

func (s *Scheduler) killJob(key string) {
	s.rMutex.Lock()
	defer s.rMutex.Unlock()
	if val, ok := s.rMap[key]; ok {
		if val.command.Process != nil {
			val.command.Process.Kill()
			val.command.Process.Wait()
		}
	}
}

func (s *Scheduler) removeJob(key string) {
	s.rMutex.Lock()
	defer s.rMutex.Unlock()
	delete(s.rMap, key)
}

func (s *Scheduler) schedulerComplete(key string) bool {
	s.rMutex.Lock()
	defer s.rMutex.Unlock()
	if val, ok := s.rMap[key]; ok {
		//Validate that this in the process queue
		if val.status == "InQueue" && len(s.commands) == 0 {
			s.commands = append(s.commands, val)
			s.processCommands()
		}

		return val.endTime > 0
	}

	// Default to true if the key is not found
	return true
}

func (s *Scheduler) processCommands() {
	s.cMutex.Lock()
	if len(s.commands) > 0 {
		c := s.commands[0]
		s.commands = s.commands[1:]
		s.lastRun = c.command.Path + " -> " + fmt.Sprintf("%v", c.command.Args)
		err := run(c)
		if err != nil {
			c.endTime = time.Now().Unix()
		}
	}
	s.cMutex.Unlock()
}

func run(c *rCommand) error {
	c.status = "Running"
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

	out, err1 := c.command.StderrPipe()
	outr, err2 := c.command.StdoutPipe()

	if c.crash1 || err1 != nil {
		return err1
	}

	if c.crash2 || err2 != nil {
		return err2
	}

	scanner := bufio.NewScanner(out)
	go func() {
		for scanner != nil && scanner.Scan() {
			c.output += scanner.Text()
		}
		out.Close()
	}()

	scanner2 := bufio.NewScanner(outr)
	go func() {
		for scanner2 != nil && scanner2.Scan() {
			c.mainOut += scanner2.Text()
		}
		outr.Close()
	}()

	c.status = "StartCommand"
	err := c.command.Start()
	if err != nil {
		return err
	}
	c.startTime = time.Now().Unix()

	// Monitor the job and report completion
	go func() {
		c.status = "Entering Wait"
		err := c.command.Wait()
		c.status = "Completed Wait"
		if err != nil {
			c.err = err
		}
		c.endTime = time.Now().Unix()
	}()

	return nil
}
