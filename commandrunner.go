package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	pb "github.com/brotherlogic/gobuildslave/proto"
)

const (
	waitTime  = time.Second
	pauseTime = 10 * time.Millisecond
)

type disker interface {
	getDisks() []string
}

type diskChecker interface {
	diskUsage(path string) int64
}

type prodDiskChecker struct{}

func diskUsage(path string) int64 {
	fs := syscall.Statfs_t{}
	err := syscall.Statfs(path, &fs)
	if err != nil {
		return -1
	}
	return int64(fs.Bfree * uint64(fs.Bsize))
}

// Runner is the server that runs commands
type Runner struct {
	commands        []*runnerCommand
	runCommands     []*runnerCommand
	runner          func(*runnerCommand)
	gopath          string
	running         bool
	lameDuck        bool
	commandsRun     int
	backgroundTasks []*runnerCommand
	m               *sync.Mutex
	bm              *sync.Mutex
	getip           func(string) (string, int)
	logger          func(string)
}

type runnerCommand struct {
	command    *exec.Cmd
	discard    bool
	output     string
	complete   bool
	background bool
	details    *pb.JobDetails
	started    time.Time
	hash       string
}

func (r *Runner) run() {
	r.m.Lock()
	r.running = true

	for r.running {
		time.Sleep(pauseTime)
		if len(r.commands) > 0 {

			r.runner(r.commands[0])
			if r.commands[0].background {
				r.bm.Lock()
				r.backgroundTasks = append(r.backgroundTasks, r.commands[0])
				r.bm.Unlock()
			}
			r.runCommands = append(r.runCommands, r.commands[0])
			r.commands = r.commands[1:]
			r.commandsRun++
		}
	}
	r.m.Unlock()
}

func (r *Runner) kill(details *pb.JobDetails) {
	r.bm.Lock()
	for i, t := range r.backgroundTasks {
		if t.details.GetSpec().Name == details.Spec.Name {
			if t.command.Process != nil {
				t.command.Process.Kill()
				t.command.Process.Wait()
			}
			// Now deliver the crash Report
			deliverCrashReport(t, r.getip, r.logger)
			r.commandsRun++
			r.backgroundTasks = append(r.backgroundTasks[:i], r.backgroundTasks[i+1:]...)
		}
	}
	r.bm.Unlock()
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
	r.addCommand(&runnerCommand{command: exec.Command("go", "get", "-u", repo)})
	readCommand := &runnerCommand{command: exec.Command("cat", "$GOPATH/src/"+repo+"/.git/refs/heads/master"), discard: false}
	r.addCommand(readCommand)
	r.BlockUntil(readCommand)

	return readCommand.output
}

// Rebuild and rerun a JobSpec
func (r *Runner) Rebuild(details *pb.JobDetails, currentHash string) {
	r.Checkout(details.Spec.GetName())
	elems := strings.Split(details.Spec.Name, "/")
	command := elems[len(elems)-1]
	hash, err := getHash("$GOPATH/bin/" + command)
	if err != nil {
		hash = "nohash"
	}
	if hash != currentHash {
		r.logger(fmt.Sprintf("Rebuilding %v for hash difference %v->%v", details, hash, currentHash))
		details.State = pb.State_BUILT
	}
}

//Update the job with new cl args
func (r *Runner) Update(spec *pb.JobDetails) {
	r.kill(spec)
	r.Run(spec)
}

// Run the specified server specified in the repo
func (r *Runner) Run(details *pb.JobDetails) {
	elems := strings.Split(details.Spec.Name, "/")
	command := elems[len(elems)-1]

	if stat, err := os.Stat("$GOPATH/bin/" + command); os.IsNotExist(err) || time.Since(stat.ModTime()).Hours() > 1 {
		r.Checkout(details.Spec.Name)
	}

	//Kill any currently running tasks
	r.kill(details)

	hash, err := getHash("$GOPATH/bin/" + command)
	if err != nil {
		hash = "nohash"
	}

	//Prepare to runnerCommand
	details.StartTime = 0
	details.State = pb.State_BUILT

	com := &runnerCommand{command: exec.Command("$GOPATH/bin/"+command, details.Spec.Args...), background: true, details: details, started: time.Now(), hash: hash}
	r.addCommand(com)
}
