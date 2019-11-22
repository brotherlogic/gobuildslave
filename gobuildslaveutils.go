package main

import (
	"fmt"
	"os/exec"
	"time"

	pbb "github.com/brotherlogic/buildserver/proto"
	pbfc "github.com/brotherlogic/filecopier/proto"
	pb "github.com/brotherlogic/gobuildslave/proto"
	"golang.org/x/net/context"
)

const (
	pendWait = time.Minute
)

func (s *Server) runTransition(ctx context.Context, job *pb.JobAssignment) {
	startState := job.State
	switch job.State {
	case pb.State_ACKNOWLEDGED:
		key := s.scheduleBuild(ctx, job)
		s.stateMutex.Lock()
		s.stateMap[job.Job.Name] = fmt.Sprintf("SCHED: %v @ %v", key, time.Now())
		s.stateMutex.Unlock()
		if !job.Job.Bootstrap {
			if key != "" {
				job.Server = s.Registry.Identifier
				job.State = pb.State_BUILT
				job.RunningVersion = key
			} else {
				// Bootstrap this job since we don't have an initial version
				if job.Job.PartialBootstrap {
					job.Job.Bootstrap = true
				}
			}
		} else {
			job.CommandKey = key
			job.State = pb.State_BUILDING
		}
	case pb.State_BUILDING:
		s.stateMutex.Lock()
		s.stateMap[job.Job.Name] = fmt.Sprintf("BUILD(%v): %v", job.CommandKey, s.scheduler.getState(job.CommandKey))
		s.stateMutex.Unlock()
		if s.taskComplete(job.CommandKey) {
			job.State = pb.State_BUILT
		}
	case pb.State_BUILT:
		output, _ := s.scheduler.getOutput(job.CommandKey)
		s.stateMutex.Lock()
		s.stateMap[job.Job.Name] = fmt.Sprintf("BUILT(%v): (%v): %v", job.CommandKey, len(output), output)
		s.stateMutex.Unlock()
		if job.Job.Bootstrap && len(output) > 0 {
			if job.BuildFail == 5 {
				s.deliverCrashReport(ctx, job, output)
				job.BuildFail = 0
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
		if job.Job.PartialBootstrap && job.Job.Bootstrap {
			job.Job.Bootstrap = false
		}
		s.stateMutex.Lock()
		out, _ := s.scheduler.getOutput(job.CommandKey)
		s.stateMap[job.Job.Name] = fmt.Sprintf("OUTPUT = %v", out)
		s.stateMutex.Unlock()
		if time.Now().Add(-time.Minute).Unix() > job.StartTime {
			job.State = pb.State_RUNNING
		}
	case pb.State_RUNNING:
		output, errout := s.scheduler.getOutput(job.CommandKey)
		s.stateMutex.Lock()
		s.stateMap[job.Job.Name] = fmt.Sprintf("ROUTPUT = %v, %v", output, s.scheduler.getStatus(job.CommandKey))
		job.Status = s.scheduler.getStatus(job.CommandKey)
		s.stateMutex.Unlock()
		if len(job.CommandKey) > 0 && s.taskComplete(job.CommandKey) {
			s.stateMutex.Lock()
			s.stateMap[job.Job.Name] = fmt.Sprintf("COMPLETE = (%v, %v)", job, output)
			s.stateMutex.Unlock()
			s.deliverCrashReport(ctx, job, output)
			job.State = pb.State_DIED
		}

		if s.discover != nil {
			err := s.discover.discover(job.Job.Name, s.Registry.Identifier)
			if err != nil {
				if job.DiscoverCount > 30 {
					output2, errout2 := s.scheduler.getErrOutput(job.CommandKey)
					s.RaiseIssue(ctx, "Cannot Discover Running Server", fmt.Sprintf("%v on %v is not discoverable, despite running (%v) the output says %v (%v), %v, %v", job.Job.Name, s.Registry.Identifier, err, output, errout, output2, errout2), false)
				}
				job.DiscoverCount++
			} else {
				job.DiscoverCount = 0
			}
		}

		// Restart this job if we need to
		if !job.Job.Bootstrap {
			if time.Now().Sub(time.Unix(job.LastVersionPull, 0)) > time.Minute*5 {
				version, err := s.getVersion(ctx, job.Job)
				job.LastVersionPull = time.Now().Unix()

				if err == nil && version.Version != job.RunningVersion {
					s.stateMutex.Lock()
					s.stateMap[job.Job.Name] = fmt.Sprintf("VERSION_MISMATCH = %v,%v", version, job.RunningVersion)
					s.stateMutex.Unlock()
					s.scheduler.killJob(job.CommandKey)
				}
			}
		}
	case pb.State_BRINK_OF_DEATH:
		if s.version.confirm(ctx, job.Job.Name) {
			s.scheduler.killJob(job.CommandKey)
			job.State = pb.State_ACKNOWLEDGED
		}
	case pb.State_DIED:
		s.stateMutex.Lock()
		s.stateMap[job.Job.Name] = fmt.Sprintf("DIED %v", job.CommandKey)
		s.stateMutex.Unlock()
		s.scheduler.removeJob(job.CommandKey)
		job.State = pb.State_ACKNOWLEDGED
	}

	if job.State != startState {
		job.LastTransitionTime = time.Now().Unix()
	}
}

type translator interface {
	build(job *pb.Job) *exec.Cmd
	run(job *pb.Job) *exec.Cmd
}

type checker interface {
	isAlive(ctx context.Context, job *pb.JobAssignment) bool
}

func (s *Server) getVersion(ctx context.Context, job *pb.Job) (*pbb.Version, error) {
	version, err := s.builder.build(ctx, job)
	if err != nil {
		return &pbb.Version{}, err
	}

	return version, nil

}

func updateJob(err error, job *pb.JobAssignment, resp *pbfc.CopyResponse) {
	if err == nil {
		job.QueuePos = resp.IndexInQueue
	}

}

// scheduleBuild builds out the job, returning the current version
func (s *Server) scheduleBuild(ctx context.Context, job *pb.JobAssignment) string {
	if job.Job.Bootstrap {
		c := s.translator.build(job.Job)
		return s.scheduler.Schedule(&rCommand{command: c, base: job.Job.Name})
	}

	val, err := s.builder.build(ctx, job.Job)
	if err != nil {
		s.Log(fmt.Sprintf("Error on build: %v", err))
		return ""
	}
	return val.Version
}

func (s *Server) scheduleRun(job *pb.Job) string {
	//Copy over any existing new versions
	key := s.scheduler.Schedule(&rCommand{command: exec.Command("mv", "$GOPATH/bin/"+job.GetName()+".new", "$GOPATH/bin/"+job.GetName()), base: job.Name})
	for !s.taskComplete(key) {
		time.Sleep(time.Second)
	}

	c := s.translator.run(job)
	return s.scheduler.Schedule(&rCommand{command: c, base: job.Name})
}

func (s *Server) taskComplete(key string) bool {
	return s.scheduler.schedulerComplete(key)
}
