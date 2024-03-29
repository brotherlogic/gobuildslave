package main

import (
	"log"
	"os/exec"
	"testing"

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
	&pb.JobAssignment{Job: &pb.Job{Name: "blah", GoPath: "blah", Bootstrap: true}, CommandKey: "blah", State: pb.State_BUILDING},
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
	&pb.JobAssignment{Job: &pb.Job{Name: "blah", GoPath: "blah", PartialBootstrap: true, Bootstrap: true}, State: pb.State_PENDING},
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

func TestBuildFail(t *testing.T) {
	s := getTestServer()
	s.translator = &testTranslator{}
	job := &pb.JobAssignment{Job: &pb.Job{Name: "blah", GoPath: "blah", Bootstrap: true}, State: pb.State_BUILT, CommandKey: "this thing crashed"}
	for i := 0; i < 10; i++ {
		job.State = pb.State_BUILT
		s.runTransition(context.Background(), job)
		log.Printf("NOW %v", job)
	}

	if job.State != pb.State_DIED {
		t.Errorf("Multiple failures did not fail: %v", job.State)
	}
}

func TestMoveFromTheBrink(t *testing.T) {
	s := getTestServer()
	//s.discover = &testDiscover{fail: true}
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

func TestSchedule(t *testing.T) {
	s := getTestServer()
	s.builder = &testBuilder{fail: true}

	val := s.scheduleBuild(context.Background(), &pb.JobAssignment{Job: &pb.Job{Name: "blah", GoPath: "blah"}, State: pb.State_ACKNOWLEDGED})

	if val != "" {
		t.Errorf("Returned non-blank: %v", val)
	}
}

func TestPartialChange(t *testing.T) {
	s := getTestServer()
	s.builder = &testBuilder{fail: true}

	joba := &pb.JobAssignment{Job: &pb.Job{Name: "blah", GoPath: "blah", PartialBootstrap: true}, State: pb.State_ACKNOWLEDGED}
	s.runTransition(context.Background(), joba)

	if !joba.Job.Bootstrap {
		t.Errorf("Job did not bootstrap")
	}

}
