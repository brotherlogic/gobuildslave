package main

import (
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	pbd "github.com/brotherlogic/discovery/proto"
	pb "github.com/brotherlogic/gobuildslave/proto"
	"github.com/brotherlogic/goserver"
	"github.com/brotherlogic/keystore/client"
	"golang.org/x/net/context"
)

type testDiscover struct {
	fail bool
}

func (p *testDiscover) discover(job string, server string) error {
	if p.fail {
		return fmt.Errorf("Built to faiul")
	}
	return nil
}

type testDisker struct {
	disks []string
}

func (t *testDisker) getDisks() []string {
	return t.disks
}

func testLog(str string) {
	log.Printf(str)
}

func getTestServer() *Server {
	s := Server{}
	s.GoServer = &goserver.GoServer{}
	s.runner = InitTest()
	s.jobs = make(map[string]*pb.JobDetails)
	s.njobs = make(map[string]*pb.JobAssignment)
	s.nMut = &sync.Mutex{}
	s.Register = s
	s.Registry = &pbd.RegistryEntry{Identifier: "MadeUp"}
	s.SkipLog = true
	s.disk = prodDiskChecker{}
	s.GoServer.KSclient = *keystoreclient.GetTestClient(".testfolder")
	s.scheduler = &Scheduler{cMutex: &sync.Mutex{}, rMutex: &sync.Mutex{}, rMap: make(map[string]*rCommand), Log: testLog}
	s.disker = &testDisker{}
	s.translator = &testTranslator{}
	s.builder = &testBuilder{}
	s.doesBuild = true
	s.discover = &testDiscover{}
	s.stateMap = make(map[string]string)
	s.pendingMap = make(map[time.Weekday]map[string]int)
	s.stateTime = make(map[string]time.Time)
	return &s
}

func TestBuildJob(t *testing.T) {
	s := getTestServer()
	s.BuildJob(context.Background(), &pb.JobSpec{})
}

func TestDoubleRunAPI(t *testing.T) {
	s := getTestServer()
	s.Run(context.Background(), &pb.JobSpec{Name: "test1"})
}

func TestKillJob(t *testing.T) {
	s := getTestServer()
	s.Run(context.Background(), &pb.JobSpec{Name: "test7"})
	s.Kill(context.Background(), &pb.JobSpec{Name: "test7"})
	s.List(context.Background(), &pb.Empty{})
	s.Update(context.Background(), &pb.JobSpec{Name: "test7"})
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
