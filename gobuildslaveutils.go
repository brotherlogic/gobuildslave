package main

import (
	"os/exec"

	pb "github.com/brotherlogic/gobuildslave/proto"
)

func (s *Server) runTransition(job *pb.JobAssignment) {
	switch job.State {
	case pb.State_ACKNOWLEDGED:
		s.scheduleBuild(job.Job)
		job.State = pb.State_BUILDING
	}
}

type translator interface {
	build(job *pb.Job) *exec.Cmd
}

func (s *Server) scheduleBuild(job *pb.Job) {
	c := s.translator.build(job)
	s.scheduler.Schedule(&rCommand{command: c})
}
