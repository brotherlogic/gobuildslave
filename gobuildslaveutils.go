package main

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/brotherlogic/goserver/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	pbb "github.com/brotherlogic/buildserver/proto"
	pbfc "github.com/brotherlogic/filecopier/proto"
	pb "github.com/brotherlogic/gobuildslave/proto"
	pbvt "github.com/brotherlogic/versiontracker/proto"
)

const (
	pendWait = time.Minute
)

var (
	ackQueueLen = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gobuildslave_ackqueuelen",
		Help: "The size of the ack queue",
	})
)

func (s *Server) procAcks() {
	for job := range s.ackChan {
		ackQueueLen.Set(float64(len(s.ackChan)))
		ctx, cancel := utils.ManualContext("gobuildslaveack", "gobuildslaveack", time.Minute, false)
		conn, err := s.FDialSpecificServer(ctx, s.Registry.GetIdentifier(), "versiontracker")
		if err != nil {
			s.ackChan <- job
		}

		client := pbvt.NewVersionTrackerServiceClient(conn)
		_, err = client.NewJob(ctx, &pbvt.NewJobRequest{Version: &pbb.Version{Job: job.GetJob()}})
		if err != nil {
			s.ackChan <- job
		}
		conn.Close()
		cancel()
	}
}

func (s *Server) runTransition(ctx context.Context, job *pb.JobAssignment) {
	fmt.Printf("TRANS: %v\n", job)
	startState := job.State
	job.LastUpdateTime = time.Now().Unix()
	switch job.State {
	case pb.State_WARMUP:
		s.ackChan <- job
		job.State = pb.State_ACKNOWLEDGED
	case pb.State_ACKNOWLEDGED:
		key := s.scheduleBuild(ctx, job)
		job.SubState = fmt.Sprintf("SCHED: %v @ %v", key, time.Now())
		if !job.Job.Bootstrap {
			if key != "" {
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
		s.scheduler.wait(job.CommandKey)
		job.State = pb.State_BUILT
	case pb.State_BUILT:
		job.SubState = "Getting Output"
		output, _ := s.scheduler.getOutput(job.CommandKey)
		job.SubState = "Entering Lock"
		s.stateMutex.Lock()
		s.stateMap[job.Job.Name] = fmt.Sprintf("BUILT(%v): (%v): %v", job.CommandKey, len(output), output)
		s.stateMutex.Unlock()
		if job.Job.Bootstrap && len(output) > 0 {
			if job.BuildFail == 5 {
				job.SubState = "Sending Report"
				s.deliverCrashReport(ctx, job, output)
				job.BuildFail = 0
			}
			job.BuildFail++
			job.State = pb.State_DIED
		} else {
			job.BuildFail = 0
			job.SubState = "Scheduling Run"
			key := s.scheduleRun(job)
			job.CommandKey = key
			job.StartTime = time.Now().Unix()
			job.State = pb.State_PENDING
			if _, ok := s.pendingMap[time.Now().Weekday()]; !ok {
				s.pendingMap[time.Now().Weekday()] = make(map[string]int)
			}
			s.pendingMap[time.Now().Weekday()][job.Job.Name]++
		}
		job.SubState = "Out of case"
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
		if len(job.CommandKey) > 0 {
			s.scheduler.wait(job.CommandKey)
			s.stateMutex.Lock()
			s.stateMap[job.Job.Name] = fmt.Sprintf("COMPLETE = (%v, %v)", job, output)
			s.stateMutex.Unlock()
			s.deliverCrashReport(ctx, job, output)
			job.State = pb.State_DIED
		}

		if s.discover != nil && s.Registry != nil {
			port, err := s.discover.discover(job.Job.Name, s.Registry.Identifier)
			if err != nil {
				if job.DiscoverCount > 30 {
					output2, errout2 := s.scheduler.getErrOutput(job.CommandKey)
					s.RaiseIssue("Cannot Discover Running Server", fmt.Sprintf("%v on %v is not discoverable, despite running (%v) the output says %v (%v), %v, %v", job.Job.Name, s.Registry.Identifier, err, output, errout, output2, errout2))
				}
				job.DiscoverCount++
			} else {
				job.Port = port
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
		// Block builds to prevent clashes
		return s.scheduler.Schedule(&rCommand{command: c, base: job.Job.Name, block: true})
	}

	val, err := s.builder.build(ctx, job.Job)
	if err != nil {
		fmt.Printf("BUILD Error: %v\n", err)
		return ""
	}
	return val.Version
}

func (s *Server) scheduleRun(job *pb.JobAssignment) string {
	//Copy over any existing new versions
	key := s.scheduler.Schedule(&rCommand{command: exec.Command("mv", "$GOPATH/bin/"+job.GetJob().GetName()+".new", "$GOPATH/bin/"+job.GetJob().GetName()), base: job.GetJob().GetName()})
	s.scheduler.wait(key)

	c := s.translator.run(job.GetJob())
	return s.scheduler.Schedule(&rCommand{command: c, base: job.GetJob().GetName()})
}
