package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type rCommand struct {
	command   *exec.Cmd
	key       string
	output    string
	startTime int64
	endTime   int64
	err       error
	mainOut   string
	status    string
	crash1    bool
	crash2    bool
	base      string
	block     bool
	comp      chan bool
}

//Scheduler the main task scheduler
type Scheduler struct {
	Log              func(string)
	blockingQueue    chan *rCommand
	nonblockingQueue chan *rCommand
	complete         []*rCommand
}

func (s *Scheduler) getState(key string) string {
	for _, c := range s.complete {
		if c.key == key {
			return fmt.Sprintf("%v -> %v", c.endTime, c.output)
		}
	}

	return "UNKNOWN"
}

// Schedule schedules a task
func (s *Scheduler) Schedule(c *rCommand) string {
	fmt.Printf("Scheduling: %v\n", c.command.Path)
	key := fmt.Sprintf("%v", time.Now().UnixNano())
	s.complete = append(s.complete, c)
	c.status = "InQueue"
	c.key = key
	c.comp = make(chan bool)
	if c.block {
		s.blockingQueue <- c
	} else {
		s.nonblockingQueue <- c
	}
	return key
}

func (s *Scheduler) getOutput(key string) (string, error) {
	for _, c := range s.complete {
		if c.key == key {
			return c.output, nil
		}
	}

	return key, fmt.Errorf("KEY NOT_IN_MAP: %v", key)
}

func (s *Scheduler) getErrOutput(key string) (string, error) {
	for _, c := range s.complete {
		if c.key == key {
			return c.mainOut, nil
		}
	}

	return "", fmt.Errorf("KEY NOT_IN_MAP: %v", key)
}

func (s *Scheduler) wait(key string) {
	for _, c := range s.complete {
		if c.key == key {
			fmt.Printf("Waiting for %v\n", key)
			<-c.comp
			fmt.Printf("Waited for %v\n", key)
			return
		}
	}
	fmt.Printf("Wait Failed for %v with %v\n", key, len(s.complete))
}

func (s *Scheduler) getStatus(key string) string {
	for _, val := range s.complete {
		if val.key == key {
			return val.status
		}
	}

	return fmt.Sprintf("KEY NOT_IN_MAP: %v", key)
}

func (s *Scheduler) killJob(key string) {
	for _, val := range s.complete {
		if val.key == key {
			if val.command.Process != nil {
				val.command.Process.Kill()
				val.command.Process.Wait()
			}
		}
	}
}

func (s *Scheduler) processBlockingCommands() {
	for c := range s.blockingQueue {
		err := run(c)
		if err != nil {
			fmt.Printf("Command failure: %v", err)
			c.endTime = time.Now().Unix()
		}
	}
}

func (s *Scheduler) processNonblockingCommands() {
	for c := range s.nonblockingQueue {
		err := run(c)
		if err != nil {
			fmt.Printf("Command failure: %v", err)
			c.endTime = time.Now().Unix()
		}
	}
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
		c.endTime = time.Now().Unix()
		c.comp <- true
		c.err = err
		return err
	}
	c.startTime = time.Now().Unix()

	// Monitor the job and report completion
	r := func() {
		c.status = "Entering Wait"
		err := c.command.Wait()
		c.status = "Completed Wait"
		if err != nil {
			c.err = err
		}
		c.endTime = time.Now().Unix()
		fmt.Printf("ENDED - Writing to channel\n")
		c.comp <- true
		fmt.Printf("COMPLETE\n")
	}

	if c.block {
		r()
	} else {
		go r()
	}

	return nil
}
