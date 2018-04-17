package main

import (
	"sync"
	"testing"

	"github.com/brotherlogic/goserver"
	"github.com/brotherlogic/keystore/client"
	"golang.org/x/net/context"

	pb "github.com/brotherlogic/gobuildslave/proto"
)

func getTestServer() *Server {
	s := Server{}
	s.GoServer = &goserver.GoServer{}
	s.runner = InitTest()
	s.jobs = make(map[string]*pb.JobDetails)
	s.njobs = make(map[string]*pb.JobAssignment)
	s.Register = s
	s.SkipLog = true
	s.disk = prodDiskChecker{}
	s.GoServer.KSclient = *keystoreclient.GetTestClient(".testfolder")
	s.scheduler = &Scheduler{cMutex: &sync.Mutex{}}
	return &s
}

func TestBuildJob(t *testing.T) {
	s := getTestServer()
	s.BuildJob(context.Background(), &pb.JobSpec{})
}

func TestDoubleRunAPI(t *testing.T) {
	s := getTestServer()
	s.Run(context.Background(), &pb.JobSpec{Name: "test1"})
	s.Run(context.Background(), &pb.JobSpec{Name: "test1"})

	list, err := s.List(context.Background(), &pb.Empty{})

	if err != nil {
		t.Fatalf("args")
	}

	if len(list.Details) != 1 {
		t.Errorf("Wrong number of jobs running: %v", list)
	}
}

func TestListJob(t *testing.T) {
	s := getTestServer()
	s.Run(context.Background(), &pb.JobSpec{})
	list, err := s.List(context.Background(), &pb.Empty{})

	if err != nil {
		t.Fatalf("Error listing jobs: %v", err)
	}

	if len(list.Details) != 1 {
		t.Errorf("Wrong number of jobs listed: %v", list)
	}
}

func TestUpdateJob(t *testing.T) {
	s := getTestServer()
	s.Run(context.Background(), &pb.JobSpec{Name: "test1"})
	s.Update(context.Background(), &pb.JobSpec{Name: "test1"})

	list, err := s.List(context.Background(), &pb.Empty{})

	if err != nil {
		t.Fatalf("Error listing jobs: %v", err)
	}

	if len(list.Details) != 1 {
		t.Errorf("Wrong number of jobs listed: %v", list)
	}
}

func TestUpdateNonJob(t *testing.T) {
	s := getTestServer()
	s.Run(context.Background(), &pb.JobSpec{Name: "test1"})
	s.Update(context.Background(), &pb.JobSpec{Name: "test2"})

	list, err := s.List(context.Background(), &pb.Empty{})

	if err != nil {
		t.Fatalf("Error listing jobs: %v", err)
	}

	if len(list.Details) != 1 {
		t.Errorf("Wrong number of jobs listed: %v", list)
	}
}

func TestKillJob(t *testing.T) {
	s := getTestServer()
	s.Run(context.Background(), &pb.JobSpec{Name: "test1"})
	s.Kill(context.Background(), &pb.JobSpec{Name: "test1"})

	list, err := s.List(context.Background(), &pb.Empty{})

	if err != nil {
		t.Fatalf("Error listing jobs: %v", err)
	}

	if len(list.Details) != 1 {
		t.Errorf("Wrong number of jobs listed: %v", list)
	}
}

func TestKillNonJob(t *testing.T) {
	s := getTestServer()
	s.Run(context.Background(), &pb.JobSpec{Name: "test1"})
	s.Kill(context.Background(), &pb.JobSpec{Name: "test2"})

	list, err := s.List(context.Background(), &pb.Empty{})

	if err != nil {
		t.Fatalf("Error listing jobs: %v", err)
	}

	if len(list.Details) != 1 {
		t.Errorf("Wrong number of jobs listed: %v", list)
	}
}

func TestGetConfig(t *testing.T) {
	s := getTestServer()
	config, err := s.GetConfig(context.Background(), &pb.Empty{})

	if err != nil {
		t.Fatalf("Unable to get server config: %v", err)
	}

	if config.GetGoVersion() == "" {
		t.Errorf("Unable to get go version: %v", config)
	}
}
