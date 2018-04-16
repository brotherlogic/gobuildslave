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
	if _, ok := s.njobs[req.GetJob().GetName()]; !ok {
		return nil, fmt.Errorf("Job was not running")
	}

	s.njobs[req.GetJob().GetName()].State = pb.State_KILLING
	return &pb.KillResponse{}, nil
}

//UpdateJob - updates the job
func (s *Server) UpdateJob(ctx context.Context, req *pb.UpdateRequest) (*pb.UpdateResponse, error) {
	if _, ok := s.njobs[req.GetJob().GetName()]; !ok {
		return nil, fmt.Errorf("Job was not running")
	}

	s.njobs[req.GetJob().GetName()].State = pb.State_UPDATE_STARTING
	return &pb.UpdateResponse{}, nil
}

// ListJobs - lists the jobs
func (s *Server) ListJobs(ctx context.Context, req *pb.ListRequest) (*pb.ListResponse, error) {
	resp := &pb.ListResponse{}
	for _, job := range s.njobs {
		resp.Jobs = append(resp.Jobs, job)
	}
	return resp, nil
}

// SlaveConfig gets the config for this slave
func (s *Server) SlaveConfig(ctx context.Context, req *pb.ConfigRequest) (*pb.ConfigResponse, error) {
	return &pb.ConfigResponse{Config: &pb.SlaveConfig{}}, nil
}
