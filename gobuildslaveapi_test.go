package main

import (
	"context"
	"testing"

	pb "github.com/brotherlogic/gobuildslave/proto"
)

func TestRunJobOnFail(t *testing.T) {
	s := getTestServer()
	_, err := s.RunJob(context.Background(), &pb.RunRequest{Job: &pb.Job{Name: "test1"}})

	if err != nil {
		t.Errorf("Error running job: %v", err)
	}
}

func TestRunJob(t *testing.T) {
	s := getTestServer()
	s.doesBuild = false
	_, err := s.RunJob(context.Background(), &pb.RunRequest{Job: &pb.Job{Name: "test1"}})

	if err == nil {
		t.Errorf("Job was built?")
	}
}

func TestNUpdateJob(t *testing.T) {
	s := getTestServer()
	_, err := s.RunJob(context.Background(), &pb.RunRequest{Job: &pb.Job{Name: "test1"}})
	_, err = s.UpdateJob(context.Background(), &pb.UpdateRequest{Job: &pb.Job{Name: "test1"}})

	if err != nil {
		t.Errorf("Error updating job: %v", err)
	}
}

func TestNUpdateNoJob(t *testing.T) {
	s := getTestServer()
	_, err := s.UpdateJob(context.Background(), &pb.UpdateRequest{Job: &pb.Job{Name: "test1"}})

	if err == nil {
		t.Errorf("Error updating job: %v", err)
	}
}

func TestNKillJob(t *testing.T) {
	s := getTestServer()
	_, err := s.RunJob(context.Background(), &pb.RunRequest{Job: &pb.Job{Name: "test1"}})
	_, err = s.KillJob(context.Background(), &pb.KillRequest{Job: &pb.Job{Name: "test1"}})

	if err != nil {
		t.Errorf("Error running job: %v", err)
	}
}

func TestNKillJobNotRunning(t *testing.T) {
	s := getTestServer()
	_, err := s.KillJob(context.Background(), &pb.KillRequest{Job: &pb.Job{Name: "test1"}})

	if err == nil {
		t.Errorf("Error running job: %v", err)
	}
}

func TestDoubleRunJob(t *testing.T) {
	s := getTestServer()
	_, err := s.RunJob(context.Background(), &pb.RunRequest{Job: &pb.Job{Name: "test1"}})
	_, err = s.RunJob(context.Background(), &pb.RunRequest{Job: &pb.Job{Name: "test1"}})

	if err != nil {
		t.Errorf("Error running job: %v", err)
	}
}

func TestListJobs(t *testing.T) {
	s := getTestServer()
	_, err := s.RunJob(context.Background(), &pb.RunRequest{Job: &pb.Job{Name: "test1"}})

	if err != nil {
		t.Fatalf("Unable to run job: %v", err)
	}

	list, err := s.ListJobs(context.Background(), &pb.ListRequest{})
	if err != nil {
		t.Fatalf("Unable to list jobs: %v", err)
	}

	if len(list.Jobs) != 1 {
		t.Errorf("Problem in the listing")
	}
}

func TestGetSlaveConfig(t *testing.T) {
	s := getTestServer()
	s.Registry.Identifier = "stationone"
	s.disker = &testDisker{disks: []string{"disk1"}}
	config, err := s.SlaveConfig(context.Background(), &pb.ConfigRequest{})
	if err != nil {
		t.Fatalf("Error getting config: %v", err)
	}

	if len(config.Config.Requirements) != 4 {
		t.Errorf("Requirements not been captured: %v", config)
	}
}

func TestRegMatch(t *testing.T) {
	result := `wlan0     IEEE 802.11  ESSID:SiFi
          Mode:Managed  Frequency:5.745 GHz  Access Point: 70:3A:CB:17:CF:BB
          Bit Rate=433.3 Mb/s   Tx-Power=31 dBm
          Retry short limit:7   RTS thr:off   Fragment thr:off
          Power Management:on
          Link Quality=68/70  Signal level=-42 dBm
          Rx invalid nwid:0  Rx invalid crypt:0  Rx invalid frag:0
          Tx excessive retries:11684  Invalid misc:0   Missed beacon:0

lo        no wireless extensions.

eth0      no wireless extensions.`
	match, match2 := extractBitRate(result)
	if match != "433.3" || match2 != "70:3A:CB:17:CF:BB" {
		t.Errorf("Mismatch :%v:%v:", match, match2)
	}
}
