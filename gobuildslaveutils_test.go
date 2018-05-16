package main

import (
	"os/exec"
	"testing"

	pb "github.com/brotherlogic/gobuildslave/proto"
)

type testTranslator struct{}

func (t *testTranslator) build(job *pb.Job) *exec.Cmd {
	return exec.Command("ls")
}

func (t *testTranslator) run(job *pb.Job) *exec.Cmd {
	return exec.Command("ls")
}

type testChecker struct {
	alive bool
}

func (t *testChecker) isAlive(job *pb.JobAssignment) bool {
	return t.alive
}

var transitionTable = []struct {
	job      *pb.JobAssignment
	complete string
	newState pb.State
	alive    bool
}{{
	&pb.JobAssignment{Job: &pb.Job{GoPath: "blah"}, State: pb.State_ACKNOWLEDGED},
	"",
	pb.State_BUILDING,
	true,
}, {
	&pb.JobAssignment{Job: &pb.Job{Name: "blah", GoPath: "blah"}, State: pb.State_BUILDING},
	"blah-build",
	pb.State_BUILT,
	true,
}, {
	&pb.JobAssignment{Job: &pb.Job{Name: "blah", GoPath: "blah"}, State: pb.State_BUILT},
	"",
	pb.State_PENDING,
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
	&pb.JobAssignment{Job: &pb.Job{Name: "blah", GoPath: "blah"}, State: pb.State_RUNNING},
	"blah-run",
	pb.State_DIED,
	false,
}}

func TestTransitions(t *testing.T) {
	s := getTestServer()
	s.translator = &testTranslator{}
	for _, test := range transitionTable {
		s.scheduler.markComplete(test.complete)
		s.checker = &testChecker{alive: test.alive}
		s.runTransition(test.job)

		if test.job.State != test.newState {
			t.Errorf("Job transition failed: %v", test.job)
		}
	}
}
