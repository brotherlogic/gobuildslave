package main

import (
	"fmt"

	"golang.org/x/net/context"

	pb "github.com/brotherlogic/gobuildslave/proto"
)

// RunJob - runs the job
func (s *Server) RunJob(ctx context.Context, req *pb.RunRequest) (*pb.RunResponse, error) {
	if !s.doesBuild {
		return &pb.RunResponse{}, fmt.Errorf("Refusing to build")
	}
	s.nMut.Lock()
	defer s.nMut.Unlock()
	if _, ok := s.njobs[req.GetJob().GetName()]; ok {
		return &pb.RunResponse{}, nil
	}

	s.njobs[req.GetJob().GetName()] = &pb.JobAssignment{Job: req.GetJob(), State: pb.State_ACKNOWLEDGED}
	go s.nmonitor(s.njobs[req.GetJob().GetName()])

	return &pb.RunResponse{}, nil
}

// KillJob - kills the job
func (s *Server) KillJob(ctx context.Context, req *pb.KillRequest) (*pb.KillResponse, error) {
	s.nMut.Lock()
	defer s.nMut.Unlock()

	if _, ok := s.njobs[req.GetJob().GetName()]; !ok {
		return nil, fmt.Errorf("Job was not running")
	}

	s.njobs[req.GetJob().GetName()].State = pb.State_KILLING
	return &pb.KillResponse{}, nil
}

//UpdateJob - updates the job
func (s *Server) UpdateJob(ctx context.Context, req *pb.UpdateRequest) (*pb.UpdateResponse, error) {
	s.nMut.Lock()
	defer s.nMut.Unlock()
	if _, ok := s.njobs[req.GetJob().GetName()]; !ok {
		return nil, fmt.Errorf("Job was not running")
	}

	s.njobs[req.GetJob().GetName()].State = pb.State_UPDATE_STARTING
	return &pb.UpdateResponse{}, nil
}

// ListJobs - lists the jobs
func (s *Server) ListJobs(ctx context.Context, req *pb.ListRequest) (*pb.ListResponse, error) {
	s.nMut.Lock()
	defer s.nMut.Unlock()
	resp := &pb.ListResponse{}
	for _, job := range s.njobs {
		resp.Jobs = append(resp.Jobs, job)
	}
	return resp, nil
}

// SlaveConfig gets the config for this slave
func (s *Server) SlaveConfig(ctx context.Context, req *pb.ConfigRequest) (*pb.ConfigResponse, error) {
	disks := s.disker.getDisks()
	requirements := make([]*pb.Requirement, 0)
	for _, disk := range disks {
		requirements = append(requirements, &pb.Requirement{Category: pb.RequirementCategory_DISK, Properties: disk})
	}
	requirements = append(requirements, &pb.Requirement{Category: pb.RequirementCategory_SERVER, Properties: s.Registry.Identifier})
	if s.Registry.Identifier == "stationone" {
		requirements = append(requirements, &pb.Requirement{Category: pb.RequirementCategory_EXTERNAL, Properties: "external_ready"})
	}
	return &pb.ConfigResponse{Config: &pb.SlaveConfig{Requirements: requirements}}, nil
}
