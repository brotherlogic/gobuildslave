package main

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"reflect"
	"time"

	pbb "github.com/brotherlogic/buildserver/proto"
	pb "github.com/brotherlogic/gobuildslave/proto"
	"github.com/brotherlogic/goserver/utils"
	pbt "github.com/brotherlogic/tracer/proto"
	"github.com/golang/protobuf/proto"
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
		s.stateMutex.Lock()
		s.stateMap[job.Job.Name] = fmt.Sprintf("SCHED: %v @ %v", key, time.Now())
		s.stateMutex.Unlock()
		if !job.Job.Bootstrap {
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
		s.stateMutex.Lock()
		s.stateMap[job.Job.Name] = fmt.Sprintf("BUILD(%v): %v", job.CommandKey, s.scheduler.getState(job.CommandKey))
		s.stateMutex.Unlock()
		if s.taskComplete(job.CommandKey) {
			job.State = pb.State_BUILT
		}
	case pb.State_BUILT:
		output := s.scheduler.getOutput(job.CommandKey)
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
		s.stateMutex.Lock()
		s.stateMap[job.Job.Name] = fmt.Sprintf("OUTPUT = %v", s.scheduler.getOutput(job.CommandKey))
		s.stateMutex.Unlock()
		if time.Now().Add(-time.Minute).Unix() > job.StartTime {
			job.State = pb.State_RUNNING
		}
	case pb.State_RUNNING:
		output := s.scheduler.getOutput(job.CommandKey)
		s.stateMutex.Lock()
		s.stateMap[job.Job.Name] = fmt.Sprintf("ROUTPUT = %v, %v", s.scheduler.getOutput(job.CommandKey), s.scheduler.getStatus(job.CommandKey))
		job.Status = s.scheduler.getStatus(job.CommandKey)
		s.stateMutex.Unlock()
		if s.taskComplete(job.CommandKey) {
			s.stateMutex.Lock()
			s.stateMap[job.Job.Name] = fmt.Sprintf("COMPLETE = %v", output)
			s.stateMutex.Unlock()
			s.deliverCrashReport(ctx, job, output)
			job.State = pb.State_DIED
		}

		err := s.discover.discover(job.Job.Name, s.Registry.Identifier)
		if err != nil {
			if job.DiscoverCount > 30 {
				s.RaiseIssue(ctx, "Cannot Discover Running Server", fmt.Sprintf("%v on %v is not discoverable, despite running (%v) the output says %v", job.Job.Name, s.Registry.Identifier, err, output), false)
			}
			job.DiscoverCount++
		} else {
			job.DiscoverCount = 0
		}

		// Restart this job if we need to
		if !job.Job.Bootstrap {
			version, err := s.getVersion(ctx, job.Job)
			if err == nil && version.Version != job.RunningVersion {
				s.versions[job.Job.Name] = version
				s.stateMutex.Lock()
				s.stateMap[job.Job.Name] = fmt.Sprintf("VERSION_MISMATCH = %v,%v", version, job.RunningVersion)
				s.stateMutex.Unlock()
				s.scheduler.killJob(job.CommandKey)
				job.State = pb.State_ACKNOWLEDGED
			}
		}
	case pb.State_DIED:
		s.stateMutex.Lock()
		s.stateMap[job.Job.Name] = fmt.Sprintf("DIED %v", job.CommandKey)
		s.stateMutex.Unlock()
		s.scheduler.removeJob(job.CommandKey)
		job.State = pb.State_ACKNOWLEDGED
	}

	if job.State != stState {
		s.stateTime[job.Job.Name] = time.Now()
	}

	if job.State == pb.State_DIED {
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

func (s *Server) getVersion(ctx context.Context, job *pb.Job) (*pbb.Version, error) {
	versions, err := s.builder.build(ctx, job)
	if err != nil {
		return &pbb.Version{}, err
	}

	if len(versions) == 0 {
		return &pbb.Version{}, nil
	}

	return versions[0], nil

}

// scheduleBuild builds out the job, returning the current version
func (s *Server) scheduleBuild(ctx context.Context, job *pb.Job) string {
	utils.SendTrace(ctx, fmt.Sprintf("schedule_build_%v", job.Bootstrap), time.Now(), pbt.Milestone_MARKER, job.Name)
	if job.Bootstrap {
		c := s.translator.build(job)
		return s.scheduler.Schedule(&rCommand{command: c})
	}

	versions, err := s.builder.build(ctx, job)

	s.lastCopyStatus = fmt.Sprintf("%v", err)
	if len(versions) == 0 {
		s.Log(fmt.Sprintf("No versions for %v because %v", job.Name, err))
		s.stateMutex.Lock()
		s.stateMap[job.Name] = fmt.Sprintf("No Versions: %v", err)
		s.stateMutex.Unlock()
		return ""
	}

	t := time.Now()

	//Only copy if the latest version is different to the local version
	v, ok := s.versions[job.Name]
	if !ok || v.Version != versions[0].Version {
		s.Log(fmt.Sprintf("We're okay %v given %v", ok, reflect.ValueOf(s.versions).MapKeys()))
		s.Log(fmt.Sprintf("Requesting copy %v and %v", v, versions[0].Version))
		s.copies++

		err = s.builder.copy(ctx, versions[0])
		s.lastCopyTime = time.Now().Sub(t)
		s.lastCopyStatus = fmt.Sprintf("%v", err)
		if err != nil {
			s.stateMutex.Lock()
			s.stateMap[job.Name] = fmt.Sprintf("Copy fail (%v) -> %v", time.Now().Sub(t), err)
			s.stateMutex.Unlock()
			return ""
		}
		s.stateMutex.Lock()
		s.stateMap[job.Name] = fmt.Sprintf("Copied version %v", versions[0].Version)
		s.stateMutex.Unlock()

		//Save the version file alongside the binary
		data, _ := proto.Marshal(versions[0])
		ioutil.WriteFile("/home/simon/gobuild/bin/"+job.Name+".version", data, 0644)
	} else {
		s.skippedCopies++
	}

	return versions[0].Version
}

func (s *Server) scheduleRun(job *pb.Job) string {
	c := s.translator.run(job)
	return s.scheduler.Schedule(&rCommand{command: c})
}

func (s *Server) taskComplete(key string) bool {
	return s.scheduler.schedulerComplete(key)
}
