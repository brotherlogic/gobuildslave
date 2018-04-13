package main

import (
	"context"
	"testing"

	pb "github.com/brotherlogic/gobuildslave/proto"
)

func TestRunJob(t *testing.T) {
	s := getTestServer()
	_, err := s.RunJob(context.Background(), &pb.RunRequest{Job: &pb.Job{Name: "test1"}})

	if err != nil {
		t.Errorf("Error running job: %v", err)
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

func TestDoubleRunJob(t *testing.T) {
	s := getTestServer()
	_, err := s.RunJob(context.Background(), &pb.RunRequest{Job: &pb.Job{Name: "test1"}})
	_, err = s.RunJob(context.Background(), &pb.RunRequest{Job: &pb.Job{Name: "test1"}})

	if err != nil {
		t.Errorf("Error running job: %v", err)
	}
}

func TestWrapUp(t *testing.T) {
	s := getTestServer()
	_, err := s.KillJob(context.Background(), &pb.KillRequest{})
	if err == nil {
		t.Errorf("No need to wrap Kill")
	}
	_, err = s.SlaveConfig(context.Background(), &pb.ConfigRequest{})
	if err == nil {
		t.Errorf("No need to wrap Config")
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
