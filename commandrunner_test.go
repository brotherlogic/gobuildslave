package main

import (
	"log"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/context"

	pb "github.com/brotherlogic/gobuildslave/proto"
)

type testDiskChecker struct{}

func (diskChecker testDiskChecker) diskUsage(path string) int64 {
	if strings.HasSuffix(path, "1") {
		return int64(200)
	}

	return -1
}

func InitTest() *Runner {
	r := &Runner{}
	r.runner = testRunCommand

	go r.run()

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
	props, err := s.GetConfig(context.Background(), &pb.Empty{})

	if err != nil {
		t.Fatalf("Get Config has returned an error: %v", err)
	}

	if props.Disk == 0 || props.Memory == 0 {
		t.Errorf("Failed to pull machine details: %v", props)
	}
}

func testRunCommand(c *runnerCommand) {
	//We do nothing
	log.Printf("WHAT %v", c.command.Path)
	if strings.Contains(c.command.Path, "repols") {
		c.command.Path = "/bin/sleep"
		c.command.Args = []string{"10"}
		log.Printf("RUNNING %v", c.command)
		c.command.Start()
		log.Printf("PROC = %v", c.command.Process)
	}

	time.Sleep(200 * time.Millisecond)
	log.Printf("SLEPT")
	c.complete = true
}

func TestRun(t *testing.T) {
	r := InitTest()
	r.Run(&pb.JobSpec{Name: "testrepo"})
	r.LameDuck(true)
	if r.commandsRun != 1 {
		t.Errorf("Not enough commands: (%v) %v", r.commandsRun, r.commands)
	}
	if len(r.backgroundTasks) != 1 {
		t.Errorf("Not enough background tasks running %v", len(r.backgroundTasks))
	}
}

func TestKill(t *testing.T) {
	r := InitTest()
	r.Run(&pb.JobSpec{Name: "testrepols"})
	r.LameDuck(true)
	r.kill(&pb.JobSpec{Name: "testrepols"})

	if r.commandsRun != 1 {
		t.Errorf("Not enough commands: (%v) %v", r.commandsRun, r.commands)
	}
	if len(r.backgroundTasks) != 0 {
		t.Errorf("Not enough background tasks running %v", len(r.backgroundTasks))
	}

}

func TestCheckout(t *testing.T) {
	r := InitTest()
	log.Printf("TESTREPO CHECKOUT")
	r.Checkout("testrepo")
	log.Printf("LAMEDUCKING")
	r.LameDuck(true)

	if r.commandsRun != 2 {
		t.Errorf("Not enough commands: %v", r.commands)
	}
}
