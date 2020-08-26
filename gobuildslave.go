package main

import (
	"context"
	"fmt"
	"time"

	"github.com/brotherlogic/goserver/utils"

	dpb "github.com/brotherlogic/discovery/proto"
	pbgs "github.com/brotherlogic/goserver/proto"
)

func (s *Server) trackUpTime(ctx context.Context) error {
	conn, err := s.DialMaster("discover")
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pbgs.NewGoserverServiceClient(conn)
	state, err := client.State(ctx, &pbgs.Empty{})

	if err != nil {
		return err
	}

	for _, st := range state.GetStates() {
		if st.Key == "startup_time" {
			s.discoverStartup = time.Unix(st.TimeValue, 0)
			return nil
		}
	}

	return fmt.Errorf("No change made: %v -> %v", err, state)
}

func (s *Server) runOnChange() error {
	ctx, cancel := utils.ManualContext("gbsrr", "gbsrr", time.Minute, false)
	defer cancel()
	s.Log(fmt.Sprintf("Resyncing"))
	_, err := s.Reregister(ctx, &pbgs.ReregisterRequest{})
	if err != nil {
		return err
	}
	for _, job := range s.njobs {
		if job.Port > 0 {
			conn, err := s.DoDial(&dpb.RegistryEntry{Ip: s.Registry.Ip, Port: job.Port})
			if err != nil {
				s.Log(fmt.Sprintf("Cannot dial %v,%v -> %v", s.Registry.Ip, job.Port, err))
				break
			}
			defer conn.Close()
			client := pbgs.NewGoserverServiceClient(conn)
			_, err = client.Reregister(ctx, &pbgs.ReregisterRequest{})
			if err != nil {
				s.Log(fmt.Sprintf("Reregister failed: %v", err))
			}
		}
	}

	return nil
}

func (s *Server) unregisterChildren() error {
	ctx, cancel := utils.BuildContext("gobuildslave-unreg", "gobuildslave")
	defer cancel()

	conn, err := s.DoDial(&dpb.RegistryEntry{Ip: utils.RegistryIP, Port: utils.RegistryPort})
	if err != nil {
		return err
	}
	defer conn.Close()

	client := dpb.NewDiscoveryServiceV2Client(conn)
	_, err = client.Unregister(ctx, &dpb.UnregisterRequest{Service: &dpb.RegistryEntry{Identifier: s.Registry.Identifier}})

	return err
}
