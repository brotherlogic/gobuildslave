package main

import (
	"fmt"
	"runtime"
	"strconv"
	"time"

	pb "github.com/brotherlogic/gobuildslave/proto"
	"golang.org/x/net/context"
)

// BuildJob builds out a job
func (s *Server) BuildJob(ctx context.Context, in *pb.JobSpec) (*pb.Empty, error) {
	s.runner.Checkout(in.Name)
	return &pb.Empty{}, nil
}

// List lists all running jobs
func (s *Server) List(ctx context.Context, in *pb.Empty) (*pb.JobList, error) {
	details := &pb.JobList{}
	for _, job := range s.jobs {
		details.Details = append(details.Details, job)
	}

	return details, nil
}

// Run starts up a job
func (s *Server) Run(ctx context.Context, in *pb.JobSpec) (*pb.Empty, error) {
	t := time.Now()
	if _, ok := s.jobs[in.GetName()]; ok {
		s.LogFunction("Run-found", t)
		return &pb.Empty{}, nil
	}

	s.jobs[in.GetName()] = &pb.JobDetails{Spec: in, State: pb.State_ACKNOWLEDGED}
	go s.monitor(s.jobs[in.GetName()])

	s.LogFunction("Run-notfound", t)
	return &pb.Empty{}, nil
}

//Update restarts a job with new settings
func (s *Server) Update(ctx context.Context, in *pb.JobSpec) (*pb.Empty, error) {
	t := time.Now()

	//Only update if we're running
	if j, ok := s.jobs[in.GetName()]; !ok || j.State == pb.State_RUNNING {
		s.LogFunction("Update-notrunning", t)
		return &pb.Empty{}, fmt.Errorf("Unable to update - job not running")
	}

	s.jobs[in.GetName()].State = pb.State_UPDATE_STARTING

	s.LogFunction("Update", t)
	return &pb.Empty{}, nil
}

// Kill a background task
func (s *Server) Kill(ctx context.Context, in *pb.JobSpec) (*pb.Empty, error) {
	t := time.Now()
	//Only update if we're running
	if j, ok := s.jobs[in.GetName()]; !ok || j.State == pb.State_RUNNING {
		s.LogFunction("Kill-notrunning", t)
		return &pb.Empty{}, fmt.Errorf("No job running like that")
	}

	s.jobs[in.GetName()].State = pb.State_KILLING

	s.LogFunction("Kill", t)
	return &pb.Empty{}, nil
}

// GetConfig gets the status of the server
func (s *Server) GetConfig(ctx context.Context, in *pb.Empty) (*pb.Config, error) {
	m := &runtime.MemStats{}
	runtime.ReadMemStats(m)

	// Basic disk allowance is 100 bytes
	disk := int64(100)

	// Disks should be mounted disk1, disk2, disk3, ...
	pcount := 1
	dir := "/media/disk" + strconv.Itoa(pcount)
	found := false
	for !found {
		diskadd := int64(s.disk.diskUsage(dir))
		if diskadd < 0 {
			found = true
		} else {
			disk += diskadd
		}
		pcount++
		dir = "/media/disk" + strconv.Itoa(pcount)
	}

	//Get the go version
	version := runtime.Version()

	d1 := s.disk.diskUsage("/media/music")
	d2 := s.disk.diskUsage("/Users/simon/Music/home")

	return &pb.Config{SupportsCds: (d1 > 0 || d2 > 0), GoVersion: version, Memory: int64(m.Sys), Disk: int64(disk), External: s.Registry.GetIdentifier() == "runner"}, nil
}
