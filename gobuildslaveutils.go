package main

import (
	"os/exec"
	"time"

	pb "github.com/brotherlogic/gobuildslave/proto"
)

const (
	pendWait = time.Minute
)

func (s *Server) runTransition(job *pb.JobAssignment) {
	switch job.State {
	case pb.State_ACKNOWLEDGED:
		s.scheduleBuild(job.Job)
		job.State = pb.State_BUILDING
		job.Server = s.Registry.Identifier
	case pb.State_BUILDING:
		if s.taskComplete("build", job.Job) {
			job.State = pb.State_BUILT
		}
	case pb.State_BUILT:
		s.scheduleRun(job.Job)
		job.StartTime = time.Now().Unix()
		job.State = pb.State_PENDING
	case pb.State_PENDING:
		if time.Now().Add(-time.Minute).Unix() > job.StartTime {
			if s.checker.isAlive(job) {
				job.State = pb.State_RUNNING
			} else {
				job.State = pb.State_DIED
			}
		}
	case pb.State_DIED:
		job.State = pb.State_ACKNOWLEDGED
	case pb.State_RUNNING:
		if !s.checker.isAlive(job) {
			job.State = pb.State_DIED
		}
	}
}

type translator interface {
	build(job *pb.Job) *exec.Cmd
	run(job *pb.Job) *exec.Cmd
}

type checker interface {
	isAlive(job *pb.JobAssignment) bool
}

func (s *Server) scheduleBuild(job *pb.Job) {
	c := s.translator.build(job)
	s.scheduler.Schedule(job.Name+"-build", &rCommand{command: c})
}

func (s *Server) scheduleRun(job *pb.Job) {
	c := s.translator.run(job)
	s.scheduler.Schedule(job.Name+"-run", &rCommand{command: c})
}

func (s *Server) taskComplete(state string, job *pb.Job) bool {
	return s.scheduler.schedulerComplete(job.Name + "-" + state)
}
