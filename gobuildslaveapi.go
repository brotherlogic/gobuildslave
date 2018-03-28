package main

import (
	"fmt"

	"golang.org/x/net/context"

	pb "github.com/brotherlogic/gobuildslave/proto"
)

// RunJob - runs the job
func (s *Server) RunJob(ctx context.Context, req *pb.RunRequest) (*pb.RunResponse, error) {
	if _, ok := s.njobs[req.GetJob().GetName()]; ok {
		return &pb.RunResponse{}, nil
	}

	s.njobs[req.GetJob().GetName()] = &pb.JobAssignment{Job: req.GetJob(), State: pb.State_ACKNOWLEDGED}
	go s.nmonitor(s.njobs[req.GetJob().GetName()])

	return &pb.RunResponse{}, nil
}

// KillJob - kills the job
func (s *Server) KillJob(ctx context.Context, req *pb.KillRequest) (*pb.KillResponse, error) {
	return &pb.KillResponse{}, fmt.Errorf("NOT IMPLEMENTED")
}

// ListJobs - lists the jobs
func (s *Server) ListJobs(ctx context.Context, req *pb.ListRequest) (*pb.ListResponse, error) {
	return &pb.ListResponse{}, fmt.Errorf("NOT IMPLEMENTED")
}

// SlaveConfig gets the config for this slave
func (s *Server) SlaveConfig(ctx context.Context, req *pb.ConfigRequest) (*pb.ConfigResponse, error) {
	return &pb.ConfigResponse{}, fmt.Errorf("NOT IMPLEMENTED")
}
