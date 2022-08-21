package main

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/brotherlogic/gobuildslave/proto"
)

// RunJob - runs the job
func (s *Server) RunJob(ctx context.Context, req *pb.RunRequest) (*pb.RunResponse, error) {
	if !req.GetJob().GetBreakout() &&
		(s.Registry.Identifier == "clust6" ||
			s.Registry.Identifier == "clust3" ||
			s.Registry.Identifier == "clust7" ||
			s.Registry.Identifier == "clust8" ||
			s.Registry.Identifier == "clust4") {
		return &pb.RunResponse{}, status.Errorf(codes.FailedPrecondition, "we only run the basic set of jobs")
	}

	if req.GetBits() > 0 && s.Bits != int(req.GetBits()) {
		return &pb.RunResponse{}, status.Errorf(codes.FailedPrecondition, "Cannot run %v bits on this server", req.GetBits())
	}

	if !s.doesBuild && !req.Job.Breakout {
		return &pb.RunResponse{}, status.Errorf(codes.FailedPrecondition, "Refusing to build")
	}
	s.nMut.Lock()
	defer s.nMut.Unlock()
	if _, ok := s.njobs[req.GetJob().GetName()]; ok {
		return &pb.RunResponse{}, fmt.Errorf("Already running this job!")
	}

	if len(s.njobs) > s.maxJobs && !req.GetJob().GetBreakout() {
		return nil, status.Errorf(codes.FailedPrecondition, "We're running %v jobs, can't run no more", len(s.njobs))
	}

	s.CtxLog(ctx, "Running %v")

	s.njobs[req.GetJob().GetName()] = &pb.JobAssignment{Job: req.GetJob(), LastTransitionTime: time.Now().Unix(), Bits: int32(s.Bits)}
	go s.nmonitor(s.njobs[req.GetJob().GetName()])

	return &pb.RunResponse{}, nil
}

// KillJob - kills the job
func (s *Server) KillJob(ctx context.Context, req *pb.KillRequest) (*pb.KillResponse, error) {
	s.nMut.Lock()
	defer s.nMut.Unlock()

	if _, ok := s.njobs[req.GetJob().GetName()]; !ok {
		return nil, fmt.Errorf("Job was not running")
	}

	s.njobs[req.GetJob().GetName()].State = pb.State_KILLING
	return &pb.KillResponse{}, nil
}

//UpdateJob - updates the job
func (s *Server) UpdateJob(ctx context.Context, req *pb.UpdateRequest) (*pb.UpdateResponse, error) {
	s.nMut.Lock()
	defer s.nMut.Unlock()
	if _, ok := s.njobs[req.GetJob().GetName()]; !ok {
		return nil, fmt.Errorf("Job was not running")
	}

	s.njobs[req.GetJob().GetName()].State = pb.State_UPDATE_STARTING
	return &pb.UpdateResponse{}, nil
}

// ListJobs - lists the jobs
func (s *Server) ListJobs(ctx context.Context, req *pb.ListRequest) (*pb.ListResponse, error) {
	s.nMut.Lock()
	defer s.nMut.Unlock()
	resp := &pb.ListResponse{}
	for _, job := range s.njobs {
		resp.Jobs = append(resp.Jobs, job)
	}
	return resp, nil
}

func extractBitRate(output string) (string, string) {
	matcher := regexp.MustCompile("Rate=(.*?) ")
	matches := matcher.FindStringSubmatch(output)

	matcher2 := regexp.MustCompile("Access Point. ([A-F0-9:]*)")
	matches2 := matcher2.FindStringSubmatch(output)
	if len(matches) > 0 && len(matches2) > 0 {
		return strings.TrimRight(matches[1], " "), strings.TrimRight(matches2[1], " ")
	}
	return "", ""
}

// SlaveConfig gets the config for this slave
func (s *Server) SlaveConfig(ctx context.Context, req *pb.ConfigRequest) (*pb.ConfigResponse, error) {
	disks := s.disker.getDisks()
	requirements := make([]*pb.Requirement, 0)
	for _, disk := range disks {
		requirements = append(requirements, &pb.Requirement{Category: pb.RequirementCategory_DISK, Properties: disk})
	}
	requirements = append(requirements, &pb.Requirement{Category: pb.RequirementCategory_SERVER, Properties: s.Registry.Identifier})
	if s.Registry.Identifier == "argon" {
		requirements = append(requirements, &pb.Requirement{Category: pb.RequirementCategory_EXTERNAL, Properties: "external_ready"})
	}

	data, err := exec.Command("/usr/bin/lsusb").Output()
	if err != nil {
		return nil, fmt.Errorf("error listing usb components: %v", err)
	}
	s.CtxLog(ctx, fmt.Sprintf("USBRES: %v", string(data)))
	if strings.Contains(string(data), "TSP100II") {
		requirements = append(requirements, &pb.Requirement{Category: pb.RequirementCategory_RECEIPT_PRINTER})
	}

	out, _ := exec.Command("/sbin/iwconfig").Output()
	br, ap := extractBitRate(string(out))
	s.accessPoint = ap
	requirements = append(requirements, &pb.Requirement{Category: pb.RequirementCategory_NETWORK, Properties: br})
	requirements = append(requirements, &pb.Requirement{Category: pb.RequirementCategory_ACCESS_POINT, Properties: ap})

	requirements = append(requirements, &pb.Requirement{Category: pb.RequirementCategory_BITS, Properties: fmt.Sprintf("%v", s.Bits)})

	out, _ = exec.Command("cat", "/sys/firmware/devicetree/base/model").Output()
	requirements = append(requirements, &pb.Requirement{Category: pb.RequirementCategory_HOST_TYPE, Properties: string(out)})

	// Add in the printer

	return &pb.ConfigResponse{Config: &pb.SlaveConfig{Requirements: requirements}}, nil
}
