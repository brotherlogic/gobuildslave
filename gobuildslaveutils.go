package main

import (
	"fmt"
	"os/exec"
	"time"

	pb "github.com/brotherlogic/gobuildslave/proto"
	"github.com/brotherlogic/goserver/utils"
	pbt "github.com/brotherlogic/tracer/proto"
	"golang.org/x/net/context"
)

const (
	pendWait = time.Minute
)

func (s *Server) runTransition(ctx context.Context, job *pb.JobAssignment) {
	utils.SendTrace(ctx, fmt.Sprintf("run_transition_%v", job.State), time.Now(), pbt.Milestone_MARKER, job.Job.Name)
	stState := job.State
	switch job.State {
	case pb.State_ACKNOWLEDGED:
		key := s.scheduleBuild(ctx, job.Job)
		if job.Job.NonBootstrap {
			if key != "" {
				job.Server = s.Registry.Identifier
				job.State = pb.State_BUILT
				job.RunningVersion = key
			}
		} else {
			job.CommandKey = key
			job.State = pb.State_BUILDING
			job.Server = s.Registry.Identifier
		}
	case pb.State_BUILDING:
		if s.taskComplete(job.CommandKey) {
			job.State = pb.State_BUILT
		}
	case pb.State_BUILT:
		output := s.scheduler.getOutput(job.CommandKey)
		if len(output) > 0 {
			if job.BuildFail == 5 {
				s.deliverCrashReport(ctx, job, output)
			}
			job.BuildFail++
			job.State = pb.State_DIED
		} else {
			job.BuildFail = 0
			key := s.scheduleRun(job.Job)
			job.CommandKey = key
			job.StartTime = time.Now().Unix()
			job.State = pb.State_PENDING
			if _, ok := s.pendingMap[time.Now().Weekday()]; !ok {
				s.pendingMap[time.Now().Weekday()] = make(map[string]int)
			}
			s.pendingMap[time.Now().Weekday()][job.Job.Name]++
		}
	case pb.State_PENDING:
		if time.Now().Add(-time.Minute).Unix() > job.StartTime {
			job.State = pb.State_RUNNING
		}
	case pb.State_RUNNING:
		if s.taskComplete(job.CommandKey) {
			output := s.scheduler.getOutput(job.CommandKey)
			s.deliverCrashReport(ctx, job, output)
			job.State = pb.State_DIED
		}

		if s.discover.discover(job.Job.Name, s.Registry.Identifier) != nil {
			s.RaiseIssue(ctx, "Cannot Discover Running Server", fmt.Sprintf("%v on %v is not discoverable, despite running", job.Job.Name, s.Registry.Identifier), false)
		}

		// Restart this job if we need to
		if job.Job.NonBootstrap {
			version := s.getVersion(ctx, job.Job)
			if version != job.RunningVersion {
				s.scheduler.killJob(job.CommandKey)
				job.State = pb.State_ACKNOWLEDGED
			}
		}
	case pb.State_DIED:
		job.State = pb.State_ACKNOWLEDGED
	}
	utils.SendTrace(ctx, fmt.Sprintf("end_transition_%v_%v", job.State, stState), time.Now(), pbt.Milestone_MARKER, job.Job.Name)
	utils.SendTrace(ctx, fmt.Sprintf("end_transition_func_%v", job.State), time.Now(), pbt.Milestone_MARKER, job.Job.Name)
}

type translator interface {
	build(job *pb.Job) *exec.Cmd
	run(job *pb.Job) *exec.Cmd
}

type checker interface {
	isAlive(ctx context.Context, job *pb.JobAssignment) bool
}

func (s *Server) getVersion(ctx context.Context, job *pb.Job) string {
	versions, _ := s.builder.build(ctx, job)

	if len(versions) == 0 {
		return ""
	}

	return versions[0].Version

}

// scheduleBuild builds out the job, returning the current version
func (s *Server) scheduleBuild(ctx context.Context, job *pb.Job) string {
	utils.SendTrace(ctx, fmt.Sprintf("schedule_build_%v", job.NonBootstrap), time.Now(), pbt.Milestone_MARKER, job.Name)
	if !job.NonBootstrap {
		c := s.translator.build(job)
		return s.scheduler.Schedule(&rCommand{command: c})
	}

	versions, err := s.builder.build(ctx, job)

	if len(versions) == 0 {
		s.stateMap[job.Name] = fmt.Sprintf("No Versions: %v", err)
		return ""
	}

	t := time.Now()
	err = s.builder.copy(ctx, versions[0])
	if err != nil {
		s.stateMap[job.Name] = fmt.Sprintf("Copy fail (%v) -> %v", time.Now().Sub(t), err)
		return ""
	}
	s.stateMap[job.Name] = fmt.Sprintf("Found version %v", versions[0].Version)
	return versions[0].Version
}

func (s *Server) scheduleRun(job *pb.Job) string {
	c := s.translator.run(job)
	return s.scheduler.Schedule(&rCommand{command: c})
}

func (s *Server) taskComplete(key string) bool {
	return s.scheduler.schedulerComplete(key)
}
