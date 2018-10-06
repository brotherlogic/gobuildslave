package main

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/context"

	pbb "github.com/brotherlogic/buildserver/proto"
	pbd "github.com/brotherlogic/discovery/proto"
	pb "github.com/brotherlogic/gobuildslave/proto"
	"github.com/brotherlogic/goserver"
)

type testDiskChecker struct{}

func (diskChecker testDiskChecker) diskUsage(path string) int64 {
	if strings.HasSuffix(path, "1") {
		return int64(200)
	}

	return -1
}

type testBuilder struct {
	count    int
	copyFail bool
	change   bool
}

func (p *testBuilder) build(job *pb.Job) []*pbb.Version {
	if p.count == 0 {
		return []*pbb.Version{}
	}
	if !p.change {
		return []*pbb.Version{&pbb.Version{Version: "test"}}
	} else {
		return []*pbb.Version{&pbb.Version{Version: "newtest"}}
	}
}

func (p *testBuilder) copy(v *pbb.Version) error {
	//Pass
	if p.copyFail {
		return fmt.Errorf("Built to fail")
	}
	return nil
}

func InitTest() *Runner {
	r := &Runner{builder: &testBuilder{count: 1}, bm: &sync.Mutex{}, m: &sync.Mutex{}, getip: func(blah string) (string, int) {
		return "", -1
	}, logger: func(blah string) {
		//Do nothing
	}}
	r.runner = testRunCommand

	return r
}

func TestDiskUsage(t *testing.T) {
	v := diskUsage("/")
	if v <= 0 {
		t.Errorf("Error getting disk usage: %v", v)
	}
}

func TestDiskUsageFail(t *testing.T) {
	v := diskUsage("/madeuppath")
	if v > 0 {
		t.Errorf("Disk usage on made up path did not fail")
	}
}

func TestGetMachineCapabilities(t *testing.T) {
	s := Server{}
	s.disk = testDiskChecker{}
	s.GoServer = &goserver.GoServer{}
	props, err := s.GetConfig(context.Background(), &pb.Empty{})

	if err != nil {
		t.Fatalf("Get Config has returned an error: %v", err)
	}

	if props.Disk == 0 || props.Memory == 0 {
		t.Errorf("Failed to pull machine details: %v", props)
	}
}

func TestGetExternalFunc(t *testing.T) {
	s := Server{}
	s.GoServer = &goserver.GoServer{}
	s.disk = testDiskChecker{}
	s.Registry = &pbd.RegistryEntry{Identifier: "runner"}

	props, err := s.GetConfig(context.Background(), &pb.Empty{})

	if err != nil {
		t.Fatalf("Get Config has error'd: %v", err)
	}

	if !props.External {
		t.Errorf("Server has not returned that it can run external")
	}
}

func testRunCommand(c *runnerCommand) {
	oldPath := c.command.Path
	oldArgs := c.command.Args
	//We do nothing
	log.Printf("RUNNING COMMAND %v", c.command.Path)
	if strings.Contains(c.command.Path, "repols") {

		c.command.Path = "/bin/sleep"
		c.command.Args = []string{"10"}
		log.Printf("RUNNING %v", c.command)
		c.command.Start()
		log.Printf("PROC = %v", c.command.Process)
	}

	time.Sleep(200 * time.Millisecond)
	log.Printf("SLEPT")
	c.command.Path = oldPath
	c.command.Args = oldArgs
	c.complete = true
}

func TestGetCrashReport(t *testing.T) {
	rc := &runnerCommand{background: true, command: exec.Command("ls", "/blahblahblah"), details: &pb.JobDetails{}}
	runCommand(rc)

	// Wait for the command to finish
	time.Sleep(time.Second)

	if rc.output == "" {
		t.Errorf("Failed to get stderr")
	}

	log.Printf("GOT: %v", rc.output)
}
