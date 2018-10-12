package main

import (
	"context"
	"testing"

	pb "github.com/brotherlogic/gobuildslave/proto"
)

func TestRunJobOnFail(t *testing.T) {
	s := getTestServer()
	_, err := s.RunJob(context.Background(), &pb.RunRequest{Job: &pb.Job{Name: "test1"}})

	if err != nil {
		t.Errorf("Error running job: %v", err)
	}
}

func TestRunJob(t *testing.T) {
	s := getTestServer()
	s.doesBuild = false
	_, err := s.RunJob(context.Background(), &pb.RunRequest{Job: &pb.Job{Name: "test1"}})

	if err == nil {
		t.Errorf("Job was built?")
	}
}

func TestNUpdateJob(t *testing.T) {
	s := getTestServer()
	_, err := s.RunJob(context.Background(), &pb.RunRequest{Job: &pb.Job{Name: "test1"}})
	_, err = s.UpdateJob(context.Background(), &pb.UpdateRequest{Job: &pb.Job{Name: "test1"}})

	if err != nil {
		t.Errorf("Error updating job: %v", err)
	}
}

func TestNUpdateNoJob(t *testing.T) {
	s := getTestServer()
	_, err := s.UpdateJob(context.Background(), &pb.UpdateRequest{Job: &pb.Job{Name: "test1"}})

	if err == nil {
		t.Errorf("Error updating job: %v", err)
	}
}

func TestNKillJob(t *testing.T) {
	s := getTestServer()
	_, err := s.RunJob(context.Background(), &pb.RunRequest{Job: &pb.Job{Name: "test1"}})
	_, err = s.KillJob(context.Background(), &pb.KillRequest{Job: &pb.Job{Name: "test1"}})

	if err != nil {
		t.Errorf("Error running job: %v", err)
	}
}

func TestNKillJobNotRunning(t *testing.T) {
	s := getTestServer()
	_, err := s.KillJob(context.Background(), &pb.KillRequest{Job: &pb.Job{Name: "test1"}})

	if err == nil {
		t.Errorf("Error running job: %v", err)
	}
}

func TestDoubleRunJob(t *testing.T) {
	s := getTestServer()
	_, err := s.RunJob(context.Background(), &pb.RunRequest{Job: &pb.Job{Name: "test1"}})
	_, err = s.RunJob(context.Background(), &pb.RunRequest{Job: &pb.Job{Name: "test1"}})

	if err != nil {
		t.Errorf("Error running job: %v", err)
	}
}

func TestListJobs(t *testing.T) {
	s := getTestServer()
	_, err := s.RunJob(context.Background(), &pb.RunRequest{Job: &pb.Job{Name: "test1"}})

	if err != nil {
		t.Fatalf("Unable to run job: %v", err)
	}

	list, err := s.ListJobs(context.Background(), &pb.ListRequest{})
	if err != nil {
		t.Fatalf("Unable to list jobs: %v", err)
	}

	if len(list.Jobs) != 1 {
		t.Errorf("Problem in the listing")
	}
}

func TestGetSlaveConfig(t *testing.T) {
	s := getTestServer()
	s.Registry.Identifier = "discover"
	s.disker = &testDisker{disks: []string{"disk1"}}
	config, err := s.SlaveConfig(context.Background(), &pb.ConfigRequest{})
	if err != nil {
		t.Fatalf("Error getting config: %v", err)
	}

	if len(config.Config.Requirements) != 3 {
		t.Errorf("Requirements not been captured: %v", config)
	}
}
