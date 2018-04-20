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

var transitionTable = []struct {
	job      *pb.JobAssignment
	complete string
	newState pb.State
}{{
	&pb.JobAssignment{Job: &pb.Job{GoPath: "blah"}, State: pb.State_ACKNOWLEDGED},
	"",
	pb.State_BUILDING,
}, {
	&pb.JobAssignment{Job: &pb.Job{Name: "blah", GoPath: "blah"}, State: pb.State_BUILDING},
	"blah-build",
	pb.State_BUILT,
}}

func TestTransitions(t *testing.T) {
	s := getTestServer()
	s.translator = &testTranslator{}
	for _, test := range transitionTable {
		s.scheduler.markComplete(test.complete)
		s.runTransition(test.job)

		if test.job.State != test.newState {
			t.Errorf("Job transition failed: %v", test.job)
		}
	}
}
