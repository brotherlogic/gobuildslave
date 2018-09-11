package main

import (
	"fmt"
	"runtime"
	"strconv"

	pb "github.com/brotherlogic/gobuildslave/proto"
	"golang.org/x/net/context"
)

// BuildJob builds out a job
func (s *Server) BuildJob(ctx context.Context, in *pb.JobSpec) (*pb.Empty, error) {
	return nil, fmt.Errorf("DEPRECATED")
}

// List lists all running jobs
func (s *Server) List(ctx context.Context, in *pb.Empty) (*pb.JobList, error) {
	return nil, fmt.Errorf("DEPRECATED")
}

// Run starts up a job
func (s *Server) Run(ctx context.Context, in *pb.JobSpec) (*pb.Empty, error) {
	return nil, fmt.Errorf("DEPRECATED")
}

//Update restarts a job with new settings
func (s *Server) Update(ctx context.Context, in *pb.JobSpec) (*pb.Empty, error) {
	return nil, fmt.Errorf("DEPRECATED")
}

// Kill a background task
func (s *Server) Kill(ctx context.Context, in *pb.JobSpec) (*pb.Empty, error) {
	return nil, fmt.Errorf("DEPRECATED")
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
