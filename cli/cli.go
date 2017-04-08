package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pbdi "github.com/brotherlogic/discovery/proto"
	pb "github.com/brotherlogic/gobuildslave/proto"
)

func findServer(name, server string) (string, int) {
	conn, _ := grpc.Dial("192.168.86.34:50055", grpc.WithInsecure())
	defer conn.Close()

	registry := pbdi.NewDiscoveryServiceClient(conn)
	rs, _ := registry.ListAllServices(context.Background(), &pbdi.Empty{})

	for _, r := range rs.Services {
		if r.Identifier == server && r.Name == name {
			return r.Ip, int(r.Port)
		}
	}

	return "", -1
}

func main() {
	buildFlags := flag.NewFlagSet("BuildServer", flag.ExitOnError)
	var name = buildFlags.String("name", "", "Name of the binary to build")
	var server = buildFlags.String("server", "", "Name of the server to build on")

	if len(os.Args) <= 1 {
		fmt.Printf("Commands: build run\n")
	} else {
		switch os.Args[1] {
		case "build":
			if err := buildFlags.Parse(os.Args[2:]); err == nil {
				host, port := findServer("gobuildslave", *server)

				conn, _ := grpc.Dial(host+":"+strconv.Itoa(port), grpc.WithInsecure())
				defer conn.Close()

				registry := pb.NewGoBuildSlaveClient(conn)
				_, err := registry.BuildJob(context.Background(), &pb.JobSpec{Name: *name})
				if err != nil {
					log.Printf("Error building job: %v", err)
				}
			}
		case "run":
			if err := buildFlags.Parse(os.Args[2:]); err == nil {
				host, port := findServer("gobuildslave", *server)

				conn, _ := grpc.Dial(host+":"+strconv.Itoa(port), grpc.WithInsecure())
				defer conn.Close()

				registry := pb.NewGoBuildSlaveClient(conn)
				_, err := registry.Run(context.Background(), &pb.JobSpec{Name: *name})
				if err != nil {
					log.Printf("Error building job: %v", err)
				}
			}
		case "list":
			if err := buildFlags.Parse(os.Args[2:]); err == nil {
				host, port := findServer("gobuildslave", *server)

				conn, _ := grpc.Dial(host+":"+strconv.Itoa(port), grpc.WithInsecure())
				defer conn.Close()

				registry := pb.NewGoBuildSlaveClient(conn)
				res, err := registry.List(context.Background(), &pb.Empty{})
				if err != nil {
					log.Printf("Error building job: %v", err)
				}
				for _, r := range res.Details {
					log.Printf("RUNNING: %v", r)
				}
			}
		}
	}
}
