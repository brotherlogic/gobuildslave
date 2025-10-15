package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/brotherlogic/goserver/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
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

// Scheduler the main task scheduler
type Scheduler struct {
	Log              func(context.Context, string)
	blockingQueue    chan *rCommand
	nonblockingQueue chan *rCommand
	complete         []*rCommand
}

func (s *Scheduler) getState(key string) string {
	for _, c := range s.complete {
		if c.key == key && c.endTime > 0 {
			return fmt.Sprintf("%v -> %v", c.endTime, c.output)
		}
	}

	return ""
}

// Schedule schedules a task
func (s *Scheduler) Schedule(c *rCommand) string {
	ctx, cancel := utils.ManualContext("gbs-schedule", time.Minute)
	defer cancel()
	key := fmt.Sprintf("%v", time.Now().UnixNano())
	s.complete = append(s.complete, c)
	c.status = "InQueue"
	c.key = key
	c.comp = make(chan bool)
	s.Log(ctx, fmt.Sprintf("running %+v with %v", c.command, c.block))
	if c.block {
		s.blockingQueue <- c
		bqSize.Set(float64(len(s.blockingQueue)))
	} else {
		s.nonblockingQueue <- c
		nbqSize.Set(float64(len(s.nonblockingQueue)))
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
			<-c.comp
			return
		}
	}
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
		bqSize.Set(float64(len(s.blockingQueue)))
		err := s.run(c)
		if err != nil {
			c.endTime = time.Now().Unix()
		}
	}
}

func (s *Scheduler) processNonblockingCommands() {
	for c := range s.nonblockingQueue {
		ctx, cancel := utils.ManualContext("gbs-pnbc", time.Minute)
		s.Log(ctx, fmt.Sprintf("Running Command: %+v", c))
		nbqSize.Set(float64(len(s.nonblockingQueue)))
		err := s.run(c)
		if err != nil {
			c.endTime = time.Now().Unix()
		}
		cancel()
	}
}

var (
	outputsize = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gobuildslave_outputsize",
		Help: "The size of the scheduler output",
	}, []string{"job", "dest"})
	nbqSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gobuildslave_nbqsize",
		Help: "The size of the scheduler output",
	})
	bqSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gobuildslave_bqsize",
		Help: "The size of the scheduler output",
	})
)

func (s *Scheduler) run(c *rCommand) error {

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
	path := fmt.Sprintf("GOPATH=%v/gobuild", home)
	pathbin := fmt.Sprintf("GOBIN=%v/gobuild/bin", home)
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
			outputsize.With(prometheus.Labels{"dest": "err", "job": c.command.Path}).Add(float64(len(scanner.Text())))
		}
		out.Close()
	}()

	scanner2 := bufio.NewScanner(outr)
	go func() {
		for scanner2 != nil && scanner2.Scan() {
			c.mainOut += scanner2.Text()
			outputsize.With(prometheus.Labels{"dest": "out", "job": c.command.Path}).Add(float64(len(scanner2.Text())))
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
		c.comp <- true
	}

	if c.block {
		r()
	} else {
		go r()
	}

	return nil
}
