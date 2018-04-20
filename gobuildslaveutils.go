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
	case pb.State_BUILDING:
		if s.taskComplete("build", job.Job) {
			job.State = pb.State_BUILT
		}
	}
}

type translator interface {
	build(job *pb.Job) *exec.Cmd
}

func (s *Server) scheduleBuild(job *pb.Job) {
	c := s.translator.build(job)
	s.scheduler.Schedule(job.Name+"-build", &rCommand{command: c})
}

func (s *Server) taskComplete(state string, job *pb.Job) bool {
	return s.scheduler.schedulerComplete(job.Name + "-" + state)
}
