package main

import (
	"os/exec"
	"time"
)

const (
	waitTime  = 100 * time.Millisecond
	pauseTime = 10 * time.Millisecond
)

// Runner is the server that runs commands
type Runner struct {
	commands    []*runnerCommand
	runner      func(*runnerCommand)
	gopath      string
	running     bool
	lameDuck    bool
	commandsRun int
}

type runnerCommand struct {
	command  *exec.Cmd
	discard  bool
	output   string
	complete bool
}

func (r *Runner) run() {
	r.running = true

	for r.running {
		time.Sleep(pauseTime)
		if len(r.commands) > 0 {
			r.runner(r.commands[0])
			r.commands = r.commands[1:]
			r.commandsRun++
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
	r.addCommand(&runnerCommand{command: exec.Command("go", "get", "-u", repo)})
	readCommand := &runnerCommand{command: exec.Command("cat", "$GOPATH/repo/refs/heads/master"), discard: false}
	r.addCommand(readCommand)

	r.BlockUntil(readCommand)
	return readCommand.output
}
