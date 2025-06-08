package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/brotherlogic/goserver/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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
		done := false
		for !done {
			ackQueueLen.Set(float64(len(s.ackChan)))
			ctx, cancel := utils.ManualContext("gobuildslaveack", time.Minute)
			conn, err := s.FDialSpecificServer(ctx, "versiontracker", s.Registry.GetIdentifier())
			if err != nil {
				s.DLog(ctx, fmt.Sprintf("Dial error: (%v), %v\n", job.GetJob(), err))
			} else {
				client := pbvt.NewVersionTrackerServiceClient(conn)
				_, err = client.NewJob(ctx, &pbvt.NewJobRequest{Version: &pbb.Version{Version: job.GetRunningVersion(), Job: job.GetJob()}})

				//Slave can block if version tracker is unavailable - ignore this failure
				if err == nil || status.Convert(err).Code() == codes.Unavailable {
					done = true
				}
				conn.Close()
			}
			cancel()

			// Don't rush the system
			time.Sleep(time.Second)
		}
	}
}

func (s *Server) runTransition(ctx context.Context, job *pb.JobAssignment) {
	// Stop all the tranistions if we're shutting down
	if s.shuttingDown {
		return
	}
	s.DLog(ctx, fmt.Sprintf("TRANS: %v\n", job))
	startState := job.State
	job.LastUpdateTime = time.Now().Unix()
	switch job.State {
	case pb.State_WARMUP:
		res, err := exec.Command("md5sum", fmt.Sprintf("/home/simon/gobuild/bin/%v", job.GetJob().GetName())).Output()
		if err != nil {
			s.CtxLog(ctx, fmt.Sprintf("Error reading md5sum: %v", err))
		} else {
			elems := strings.Fields(string(res))
			job.RunningVersion = elems[0]
		}

		// Need to ack this job to get a version
		s.ackChan <- job

		job.State = pb.State_ACKNOWLEDGED
	case pb.State_ACKNOWLEDGED:
		key := s.scheduleBuild(ctx, job)
		job.SubState = fmt.Sprintf("SCHED: %v @ %v", key, time.Now())
		if !job.Job.Bootstrap {
			if key != "" {
				job.State = pb.State_BUILT
				job.RunningVersion = key
				s.ackChan <- job
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
		s.CtxLog(ctx, fmt.Sprintf("Locking %v", job.Job.Name))
		s.stateMutex.Lock()
		s.stateMap[job.Job.Name] = fmt.Sprintf("BUILD(%v): %v", job.CommandKey, s.scheduler.getState(job.CommandKey))
		s.stateMutex.Unlock()
		s.CtxLog(ctx, fmt.Sprintf("UnLocking %v", job.Job.Name))
		s.scheduler.wait(job.CommandKey)
		job.State = pb.State_BUILT
	case pb.State_BUILT:
		job.SubState = "Getting Output"
		output, _ := s.scheduler.getOutput(job.CommandKey)
		job.SubState = "Entering Lock"
		s.CtxLog(ctx, fmt.Sprintf("Built Job %v -> %v", job.Job.Name, output))
		s.stateMutex.Lock()
		s.stateMap[job.Job.Name] = fmt.Sprintf("BUILT(%v): (%v): %v", job.CommandKey, len(output), output)
		s.stateMutex.Unlock()
		s.CtxLog(ctx, fmt.Sprintf("UnLocking %v", job.Job.Name))
		if job.Job.Bootstrap && len(output) > 0 {
			if job.BuildFail == 5 {
				job.SubState = "Sending Report"
				s.deliverCrashReport(ctx, job, output)
				job.BuildFail = 0
			}
			job.BuildFail++
			job.State = pb.State_DIED
		} else {
			job.State = pb.State_VERSION_CHECK
			// Don't check version on pb jobs
			if job.GetJob().GetPartialBootstrap() {
				s.doCopy(job)
				job.BuildFail = 0
				job.SubState = "Scheduling A Run post copy"
				key := s.scheduleRun(job)
				job.CommandKey = key
				job.StartTime = time.Now().Unix()
				job.State = pb.State_PENDING
				if _, ok := s.pendingMap[time.Now().Weekday()]; !ok {
					s.pendingMap[time.Now().Weekday()] = make(map[string]int)
				}
				s.pendingMap[time.Now().Weekday()][job.Job.Name]++
			}
		}
		job.SubState = "Out of case"
	case pb.State_VERSION_CHECK:
		s.doCopy(job)
		s.loadCurrentVersions()
		version, err := s.getLatestVersion(ctx, job.GetJob().Name, job.GetJob().GetGoPath())
		if err != nil {
			s.CtxLog(ctx, fmt.Sprintf("Error getting version: %v", err))
			break
		}

		res, err := exec.Command("md5sum", fmt.Sprintf("/home/simon/gobuild/bin/%v", job.GetJob().GetName())).Output()
		if err != nil {
			s.CtxLog(ctx, fmt.Sprintf("Error reading md5sum: %v", err))
		}
		elems := strings.Fields(string(res))
		job.RunningVersion = elems[0]
		s.ackChan <- job

		if elems[0] != version.GetVersion() {
			s.versionsMutex.Lock()
			s.CtxLog(ctx, fmt.Sprintf("Bad version on %v for %v -> %v vs %v", s.Registry.Identifier, job.GetJob().GetName(), elems[0], s.versions[job.GetJob().GetName()].Version))
			s.versionsMutex.Unlock()

			job.SubState = fmt.Sprintf("Dealing With Version Mismatch: %v", job.BuildFail)

			if job.BuildFail > 10 {
				// Do a fire and forget build request
				conn, err := s.FDialServer(ctx, "buildserver")
				if err == nil {
					bclient := pbb.NewBuildServiceClient(conn)
					bclient.Build(ctx, &pbb.BuildRequest{
						Job:     job.Job,
						BitSize: int32(s.Bits),
					})
					conn.Close()

				}
				s.RaiseIssue(fmt.Sprintf("Error running %v", job.Job.Name), fmt.Sprintf("Running on %v", s.Registry.Identifier))
			}

			conn, err := s.FDialSpecificServer(ctx, "versiontracker", s.Registry.Identifier)
			if err != nil {
				s.CtxLog(ctx, fmt.Sprintf("Unable to dial vt: %v", err))
			} else {
				defer conn.Close()
				vtc := pbvt.NewVersionTrackerServiceClient(conn)
				_, err = vtc.NewVersion(ctx, &pbvt.NewVersionRequest{
					Version: version,
				})
				s.CtxLog(ctx, fmt.Sprintf("Requested new version: %v", err))
			}

			// Don't let the job sit here
			job.BuildFail++
			if job.BuildFail > 10 {
				job.State = pb.State_ACKNOWLEDGED

			}
			break
		}

		job.BuildFail = 0
		job.SubState = "Scheduling of the Run"
		key := s.scheduleRun(job)
		job.CommandKey = key
		job.StartTime = time.Now().Unix()
		job.State = pb.State_PENDING
		if _, ok := s.pendingMap[time.Now().Weekday()]; !ok {
			s.pendingMap[time.Now().Weekday()] = make(map[string]int)
		}
		s.pendingMap[time.Now().Weekday()][job.Job.Name]++

	case pb.State_PENDING:
		res, err := exec.Command("md5sum", fmt.Sprintf("/home/simon/gobuild/bin/%v", job.GetJob().GetName())).Output()
		if err != nil {
			s.CtxLog(ctx, fmt.Sprintf("Error reading md5sum: %v", err))
		}
		s.CtxLog(ctx, fmt.Sprintf("Read md54sum for %v -> %v", job.GetJob().GetName(), string(res)))
		elems := strings.Fields(string(res))
		job.RunningVersion = elems[0]
		s.CtxLog(ctx, fmt.Sprintf("Sending to ack chan %v -> %v", job.GetJob().GetName(), len(s.ackChan)))
		s.ackChan <- job

		if job.Job.PartialBootstrap && job.Job.Bootstrap {
			job.Job.Bootstrap = false
		}
		s.CtxLog(ctx, fmt.Sprintf("Locking %v", job.Job.Name))
		s.stateMutex.Lock()
		out, _ := s.scheduler.getOutput(job.CommandKey)
		s.stateMap[job.Job.Name] = fmt.Sprintf("OUTPUT = %v", out)
		s.stateMutex.Unlock()
		s.CtxLog(ctx, fmt.Sprintf("UnLocking %v", job.Job.Name))
		if time.Now().Add(-time.Minute).Unix() > job.StartTime {
			var err error
			if job.Job.Name == "discovery" {
				err = s.runOnChange()
			}
			code := status.Convert(err).Code()

			//Unavailable allows the job to die here
			if code == codes.OK || code == codes.Unavailable {
				// Validate that the job is alive
				if !s.isJobAlive(ctx, job) {
					s.CtxLog(ctx, fmt.Sprintf("Job %v is not alive", job))
				}
				job.State = pb.State_RUNNING
			} else {
				if len(s.scheduler.getState(job.CommandKey)) > 0 {
					job.State = pb.State_DIED
					s.CtxLog(ctx, fmt.Sprintf("Recording job as dead: '%v'", s.scheduler.getState(job.CommandKey)))
				}
				s.CtxLog(ctx, fmt.Sprintf("Cannot reregister: %v", err))
			}
		}
	case pb.State_RUNNING:
		output, errout := s.scheduler.getOutput(job.CommandKey)
		output2, _ := s.scheduler.getErrOutput(job.CommandKey)
		s.CtxLog(ctx, fmt.Sprintf("Locking %v", job.Job.Name))
		s.stateMutex.Lock()
		s.stateMap[job.Job.Name] = fmt.Sprintf("ROUTPUT = %v, %v", output, s.scheduler.getStatus(job.CommandKey))
		job.Status = s.scheduler.getStatus(job.CommandKey)
		s.stateMutex.Unlock()
		s.CtxLog(ctx, fmt.Sprintf("UnLocking %v", job.Job.Name))
		if len(job.CommandKey) > 0 {
			s.scheduler.wait(job.CommandKey)
			s.stateMap[job.Job.Name] = fmt.Sprintf("ONLOCk = (%v, %v)", job, output)
			s.CtxLog(ctx, fmt.Sprintf("Locking %v", job.Job.Name))
			s.stateMutex.Lock()
			s.stateMap[job.Job.Name] = fmt.Sprintf("COMPLETE = (%v, %v)", job, output)
			s.stateMutex.Unlock()
			s.CtxLog(ctx, fmt.Sprintf("UnLocking %v", job.Job.Name))
			s.deliverCrashReport(ctx, job, fmt.Sprintf("%v%v", output, output2))
			job.State = pb.State_DIED
		}

		if s.Registry != nil {
			entry, err := s.FFindSpecificServer(ctx, job.Job.Name, s.Registry.Identifier)
			if err != nil {
				if job.DiscoverCount > 30 {
					output2, errout2 := s.scheduler.getErrOutput(job.CommandKey)
					s.RaiseIssue("Cannot Discover Running Server", fmt.Sprintf("%v on %v is not discoverable, despite running (%v) the output says %v (%v), %v, %v", job.Job.Name, s.Registry.Identifier, err, output, errout, output2, errout2))
				}
				job.DiscoverCount++
				s.CtxLog(ctx, fmt.Sprintf("Missing discover for %+v, %v -> %v", job, entry, err))
			} else {
				job.Port = entry.GetPort()
				job.DiscoverCount = 0
			}
		}

		// Restart this job if we need to
		if !job.Job.Bootstrap {
			if time.Now().Sub(time.Unix(job.LastVersionPull, 0)) > time.Minute*5 {
				version, err := s.getVersion(ctx, job.Job)
				job.LastVersionPull = time.Now().Unix()

				if err == nil && version.Version != job.RunningVersion {
					s.CtxLog(ctx, fmt.Sprintf("Locking %v", job.Job.Name))
					s.stateMutex.Lock()
					s.stateMap[job.Job.Name] = fmt.Sprintf("VERSION_MISMATCH = %v,%v", version, job.RunningVersion)
					s.stateMutex.Unlock()
					s.CtxLog(ctx, fmt.Sprintf("UnLocking %v", job.Job.Name))
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
		s.CtxLog(ctx, fmt.Sprintf("Locking %v", job.Job.Name))
		s.stateMutex.Lock()
		s.stateMap[job.Job.Name] = fmt.Sprintf("DIED %v", job.CommandKey)
		s.stateMutex.Unlock()
		s.CtxLog(ctx, fmt.Sprintf("UnLocking %v", job.Job.Name))
		job.State = pb.State_ACKNOWLEDGED
	}

	time.Sleep(time.Second * 5)

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
		s.DLog(ctx, fmt.Sprintf("BUILD Error: %v\n", err))
		return ""
	}
	return val.Version
}

func (s *Server) doCopy(job *pb.JobAssignment) {
	//Copy over any existing new versions

	key := s.scheduler.Schedule(&rCommand{command: exec.Command("mv", "$GOPATH/bin/"+job.GetJob().GetName()+".new", "$GOPATH/bin/"+job.GetJob().GetName()), base: job.GetJob().GetName()})
	s.scheduler.wait(key)

	key = s.scheduler.Schedule(&rCommand{command: exec.Command("mv", "$GOPATH/bin/"+job.GetJob().GetName()+".nversion", "$GOPATH/bin/"+job.GetJob().GetName()+".version"), base: job.GetJob().GetName() + ".version"})
	s.scheduler.wait(key)
}

func (s *Server) scheduleRun(job *pb.JobAssignment) string {
	// Wait a while before starting the job JIC
	time.Sleep(time.Second * 2)

	c := s.translator.run(job.GetJob())
	return s.scheduler.Schedule(&rCommand{command: c, base: job.GetJob().GetName()})
}
