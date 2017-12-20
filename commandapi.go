package main

import (
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

	s.jobs[in.GetName()] = &pb.JobDetails{Spec: in, State: pb.JobDetails_ACKNOWLEDGED}
	go s.monitor(s.jobs[in.GetName()])

	s.LogFunction("Run-notfound", t)
	return &pb.Empty{}, nil
}

//Update restarts a job with new settings
func (s *Server) Update(ctx context.Context, in *pb.JobSpec) (*pb.Empty, error) {
	t := time.Now()
	//Only update if we're running
	if j, ok := s.jobs[in.GetName()]; !ok || j.State == pb.JobDetails_RUNNING {
		s.LogFunction("Update-notrunning", t)
		return &pb.Empty{}, nil
	}

	s.jobs[in.GetName()].State = pb.JobDetails_UPDATE_STARTING

	s.LogFunction("Update", t)
	return &pb.Empty{}, nil
}

// Kill a background task
func (s *Server) Kill(ctx context.Context, in *pb.JobSpec) (*pb.Empty, error) {
	t := time.Now()
	//Only update if we're running
	if j, ok := s.jobs[in.GetName()]; !ok || j.State == pb.JobDetails_RUNNING {
		s.LogFunction("Kill-notrunning", t)
		return &pb.Empty{}, nil
	}

	s.jobs[in.GetName()].State = pb.JobDetails_KILLING

	s.LogFunction("Kill", t)
	return &pb.Empty{}, nil
}
