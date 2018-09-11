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
		key := s.scheduleBuild(job.Job)
		job.CommandKey = key
		job.State = pb.State_BUILDING
		job.Server = s.Registry.Identifier
	case pb.State_BUILDING:
		if s.taskComplete(job.CommandKey) {
			job.State = pb.State_BUILT
		}
	case pb.State_BUILT:
		output := s.scheduler.getOutput(job.CommandKey)
		if len(output) > 0 {
			if job.BuildFail == 5 {
				s.deliverCrashReport(job, output)
			}
			job.BuildFail++
			job.State = pb.State_DIED
		} else {
			job.BuildFail = 0
			key := s.scheduleRun(job.Job)
			job.CommandKey = key
			job.StartTime = time.Now().Unix()
			job.State = pb.State_PENDING
		}
	case pb.State_PENDING:
		if time.Now().Add(-time.Minute).Unix() > job.StartTime {
			job.State = pb.State_RUNNING
		}
	case pb.State_RUNNING:
		if s.taskComplete(job.CommandKey) {
			output := s.scheduler.getOutput(job.CommandKey)
			s.deliverCrashReport(job, output)
			job.State = pb.State_DIED
		}
	case pb.State_DIED:
		job.State = pb.State_ACKNOWLEDGED
	}
}

type translator interface {
	build(job *pb.Job) *exec.Cmd
	run(job *pb.Job) *exec.Cmd
}

type checker interface {
	isAlive(job *pb.JobAssignment) bool
}

// scheduleBuild builds out the job, returning the current version
func (s *Server) scheduleBuild(job *pb.Job) string {
	if !job.NonBootstrap {
		c := s.translator.build(job)
		return s.scheduler.Schedule(&rCommand{command: c})
	}

	versions := s.builder.build(job)

	if len(versions) == 0 {
		return ""
	}

	s.builder.copy(versions[0])
	return versions[0].Version
}

func (s *Server) scheduleRun(job *pb.Job) string {
	c := s.translator.run(job)
	return s.scheduler.Schedule(&rCommand{command: c})
}

func (s *Server) taskComplete(key string) bool {
	return s.scheduler.schedulerComplete(key)
}
