package main

import (
	"os/exec"
	"sync"
	"syscall"
	"time"

	pbb "github.com/brotherlogic/buildserver/proto"
	pbfc "github.com/brotherlogic/filecopier/proto"
	pb "github.com/brotherlogic/gobuildslave/proto"
	"golang.org/x/net/context"
)

//Builder builds out binaries
type Builder interface {
	build(ctx context.Context, job *pb.Job) (*pbb.Version, error)
	copy(ctx context.Context, v *pbb.Version) (*pbfc.CopyResponse, error)
}

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
	builder         Builder
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
