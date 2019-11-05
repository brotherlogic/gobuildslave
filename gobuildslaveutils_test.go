package main

import (
	"log"
	"os/exec"
	"testing"
	"time"

	pbfc "github.com/brotherlogic/filecopier/proto"
	pb "github.com/brotherlogic/gobuildslave/proto"
	"golang.org/x/net/context"
)

type testTranslator struct{}

func (t *testTranslator) build(job *pb.Job) *exec.Cmd {
	log.Printf("BUILDING %v", job)
	return exec.Command("ls")
}

func (t *testTranslator) run(job *pb.Job) *exec.Cmd {
	return exec.Command("ls")
}

type testChecker struct {
	alive bool
}

func (t *testChecker) isAlive(ctx context.Context, job *pb.JobAssignment) bool {
	return t.alive
}

type testVersion struct{}

func (p *testVersion) confirm(ctx context.Context, job string) bool {
	return true
}

var transitionTable = []struct {
	job      *pb.JobAssignment
	complete string
	newState pb.State
	alive    bool
}{{
	&pb.JobAssignment{Job: &pb.Job{GoPath: "blah", Bootstrap: true}, State: pb.State_ACKNOWLEDGED},
	"",
	pb.State_BUILDING,
	true,
}, {
	&pb.JobAssignment{Job: &pb.Job{Name: "blah", GoPath: "blah", Bootstrap: true}, State: pb.State_BUILDING},
	"blah-build",
	pb.State_BUILT,
	true,
}, {
	&pb.JobAssignment{Job: &pb.Job{Name: "blah", GoPath: "blah"}, State: pb.State_BUILT},
	"",
	pb.State_PENDING,
	true,
}, {
	&pb.JobAssignment{Job: &pb.Job{Name: "blah", GoPath: "blah", Bootstrap: true}, CommandKey: "this thing crashed", State: pb.State_BUILT},
	"this thing crashed",
	pb.State_DIED,
	true,
}, {
	&pb.JobAssignment{Job: &pb.Job{Name: "blah", GoPath: "blah"}, State: pb.State_PENDING},
	"",
	pb.State_RUNNING,
	true,
}, {
	&pb.JobAssignment{Job: &pb.Job{Name: "blah", GoPath: "blah"}, State: pb.State_DIED},
	"",
	pb.State_ACKNOWLEDGED,
	false,
}, {
	&pb.JobAssignment{Job: &pb.Job{Name: "blah", GoPath: "blah"}, State: pb.State_ACKNOWLEDGED},
	"blah-run",
	pb.State_ACKNOWLEDGED,
	false,
}}

func TestBadBuild(t *testing.T) {
	s := getTestServer()
	s.builder = &testBuilder{fail: true}

	_, err := s.getVersion(context.Background(), &pb.Job{Name: "blah"})
	if err == nil {
		t.Errorf("Bad builder did not fail")
	}
}

func TestTransitions(t *testing.T) {
	s := getTestServer()
	s.translator = &testTranslator{}
	for _, test := range transitionTable {
		s.scheduler.markComplete(test.complete)
		s.checker = &testChecker{alive: test.alive}
		s.runTransition(context.Background(), test.job)

		if test.job.State != test.newState {
			t.Errorf("Job transition failed: %v should have been %v but was %v", test.job, test.newState, test.job.State)
		}
	}
}

func TestBuildFail(t *testing.T) {
	s := getTestServer()
	s.translator = &testTranslator{}
	job := &pb.JobAssignment{Job: &pb.Job{Name: "blah", GoPath: "blah", Bootstrap: true}, State: pb.State_BUILT, CommandKey: "this thing crashed"}
	for i := 0; i < 10; i++ {
		job.State = pb.State_BUILT
		s.scheduler.markComplete("this thing crashed")
		s.runTransition(context.Background(), job)
		log.Printf("NOW %v", job)
	}

	if job.State != pb.State_DIED {
		t.Errorf("Multiple failures did not fail: %v", job.State)
	}
}

func TestFailDiscover(t *testing.T) {
	s := getTestServer()
	s.discover = &testDiscover{fail: true}
	job := &pb.JobAssignment{Job: &pb.Job{Name: "blah", GoPath: "blah"}, State: pb.State_RUNNING}
	for i := 0; i < 32; i++ {
		s.runTransition(context.Background(), job)
	}
}

func TestMoveFromTheBrink(t *testing.T) {
	s := getTestServer()
	s.discover = &testDiscover{fail: true}
	job := &pb.JobAssignment{Job: &pb.Job{Name: "blah", GoPath: "blah"}, State: pb.State_BRINK_OF_DEATH}
	s.runTransition(context.Background(), job)
}

func TestResp(t *testing.T) {
	updateJob(nil, &pb.JobAssignment{}, &pbfc.CopyResponse{})
}

func TestBuildJobBaseTrans(t *testing.T) {
	s := getTestServer()
	s.builder = &testBuilder{count: 2}
	job := &pb.JobAssignment{Job: &pb.Job{Name: "blah", GoPath: "blah"}, State: pb.State_ACKNOWLEDGED}
	s.runTransition(context.Background(), job)

	if job.State != pb.State_BUILT {
		t.Errorf("Multiple failures did not fail: %v", job.State)
	}
}

func TestKill(t *testing.T) {
	s := getTestServer()
	s.builder = &testBuilder{count: 2}
	job := &pb.JobAssignment{Job: &pb.Job{Name: "blah", GoPath: "blah"}, State: pb.State_ACKNOWLEDGED}
	s.runTransition(context.Background(), job)

	if job.State != pb.State_BUILT {
		t.Errorf("Was not built")
	}

	s.runTransition(context.Background(), job)
	log.Printf("NOW %v", job.State)

	time.Sleep(time.Minute * 2)
	s.runTransition(context.Background(), job)
	log.Printf("NOW %v", job.State)

	s.builder = &testBuilder{change: true, count: 2}
	s.runTransition(context.Background(), job)
	log.Printf("NOW %v", job.State)
}
