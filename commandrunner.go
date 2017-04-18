package main

import (
	"log"
	"os/exec"
	"strings"
	"time"

	pb "github.com/brotherlogic/gobuildslave/proto"
)

const (
	waitTime  = 100 * time.Millisecond
	pauseTime = 10 * time.Millisecond
)

// Runner is the server that runs commands
type Runner struct {
	commands        []*runnerCommand
	runner          func(*runnerCommand)
	gopath          string
	running         bool
	lameDuck        bool
	commandsRun     int
	backgroundTasks []*runnerCommand
}

type runnerCommand struct {
	command    *exec.Cmd
	discard    bool
	output     string
	complete   bool
	background bool
	details    *pb.JobDetails
}

func (r *Runner) run() {
	r.running = true

	for r.running {
		time.Sleep(pauseTime)
		if len(r.commands) > 0 {
			r.runner(r.commands[0])
			if r.commands[0].background {
				r.backgroundTasks = append(r.backgroundTasks, r.commands[0])
			}
			r.commands = r.commands[1:]
			r.commandsRun++
		}
	}
}

func (r *Runner) kill(spec *pb.JobSpec) {
	for i, t := range r.backgroundTasks {
		log.Printf("HERE : %v, %v", i, t)
		if t.details.GetSpec().Name == spec.Name {
			log.Printf("KILL: %v", t.command.Process)
			if t.command.Process != nil {
				t.command.Process.Kill()
			}
			r.backgroundTasks = append(r.backgroundTasks[:i], r.backgroundTasks[i+1:]...)
		}
	}
}

// BlockUntil blocks on this until the command has run
func (r *Runner) BlockUntil(command *runnerCommand) {
	for !command.complete {
		time.Sleep(waitTime)
	}
}

// LameDuck the server
func (r *Runner) LameDuck(shutdown bool) {
	r.lameDuck = true

	for len(r.commands) > 0 {
		time.Sleep(waitTime)
	}

	if shutdown {
		r.running = false
	}
}

func (r *Runner) addCommand(command *runnerCommand) {
	if !r.lameDuck {
		r.commands = append(r.commands, command)
	}
}

// Checkout a repo - returns the repo version
func (r *Runner) Checkout(repo string) string {
	log.Printf("CHECKOUT = %v", repo)
	r.addCommand(&runnerCommand{command: exec.Command("go", "get", "-u", repo)})
	readCommand := &runnerCommand{command: exec.Command("cat", "$GOPATH/src/"+repo+"/.git/refs/heads/master"), discard: false}
	r.addCommand(readCommand)
	r.BlockUntil(readCommand)

	return readCommand.output
}

// Run the specified server specified in the repo
func (r *Runner) Run(spec *pb.JobSpec) {
	elems := strings.Split(spec.Name, "/")
	command := elems[len(elems)-1]
	com := &runnerCommand{command: exec.Command("$GOPATH/bin/" + command), background: true, details: &pb.JobDetails{Spec: spec}}
	r.addCommand(com)
}
