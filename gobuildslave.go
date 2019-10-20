package main

import (
	"time"

	"golang.org/x/net/context"

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

	for _, st := range state.GetStates() {
		if st.Key == "startup_time" {
			s.discoverStartup = time.Unix(st.Value, 0)
		}
	}

	return nil
}
