package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pbdi "github.com/brotherlogic/discovery/proto"
	pb "github.com/brotherlogic/gobuildslave/proto"
	"github.com/brotherlogic/goserver/utils"

	//Needed to pull in gzip encoding init
	_ "google.golang.org/grpc/encoding/gzip"
)

func findServer(name, server string) (string, int) {
	conn, _ := grpc.Dial(utils.Discover, grpc.WithInsecure())
	defer conn.Close()

	registry := pbdi.NewDiscoveryServiceClient(conn)
	rs, _ := registry.ListAllServices(context.Background(), &pbdi.ListRequest{})

	for _, r := range rs.GetServices().Services {
		if r.Identifier == server && r.Name == name {
			return r.Ip, int(r.Port)
		}
	}

	return "", -1
}

func findServers() []*pbdi.RegistryEntry {
	conn, _ := grpc.Dial(utils.Discover, grpc.WithInsecure())
	defer conn.Close()

	registry := pbdi.NewDiscoveryServiceClient(conn)
	rs, _ := registry.ListAllServices(context.Background(), &pbdi.ListRequest{})

	list := make([]*pbdi.RegistryEntry, 0)
	for _, r := range rs.GetServices().Services {
		if r.Name == "gobuildslave" {
			list = append(list, r)
		}
	}

	return list
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
					log.Fatalf("Error building job: %v", err)
				}
			}
		case "run":
			if err := buildFlags.Parse(os.Args[2:]); err == nil {
				host, port := findServer("gobuildslave", *server)

				conn, _ := grpc.Dial(host+":"+strconv.Itoa(port), grpc.WithInsecure())
				defer conn.Close()

				registry := pb.NewGoBuildSlaveClient(conn)
				_, err := registry.Run(context.Background(), &pb.JobSpec{Name: *name, Server: *server})
				if err != nil {
					log.Fatalf("Error building job: %v", err)
				}
			}
		case "kill":
			if err := buildFlags.Parse(os.Args[2:]); err == nil {
				host, port := findServer("gobuildslave", *server)

				conn, _ := grpc.Dial(host+":"+strconv.Itoa(port), grpc.WithInsecure())
				defer conn.Close()

				registry := pb.NewGoBuildSlaveClient(conn)
				_, err := registry.Kill(context.Background(), &pb.JobSpec{Name: *name, Server: *server})
				if err != nil {
					log.Fatalf("Error building job: %v", err)
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
					log.Fatalf("Error building job: %v", err)
				}
				for _, r := range res.Details {
					fmt.Printf("%v (%v) - %v\n", r.Spec.Name, time.Unix(r.StartTime, 0).Format("02/01 15:04"), r)
				}
			}
		case "nlist":
			if err := buildFlags.Parse(os.Args[2:]); err == nil {
				host, port := findServer("gobuildslave", *server)

				conn, _ := grpc.Dial(host+":"+strconv.Itoa(port), grpc.WithInsecure())
				defer conn.Close()

				registry := pb.NewBuildSlaveClient(conn)
				res, err := registry.ListJobs(context.Background(), &pb.ListRequest{})
				if err != nil {
					log.Fatalf("Error listing job: %v", err)
				}
				for _, r := range res.Jobs {
					fmt.Printf("%v -> %v\n", r.Job.Name, r.State)
				}
			}
		case "nbuild":
			if err := buildFlags.Parse(os.Args[2:]); err == nil {
				host, port := findServer("gobuildslave", *server)

				conn, _ := grpc.Dial(host+":"+strconv.Itoa(port), grpc.WithInsecure())
				defer conn.Close()

				registry := pb.NewBuildSlaveClient(conn)
				_, err := registry.RunJob(context.Background(), &pb.RunRequest{Job: &pb.Job{Name: "github.com/brotherlogic/crasher"}})
				if err != nil {
					log.Fatalf("Error listing job: %v", err)
				}
			}
		case "nconfig":
			if err := buildFlags.Parse(os.Args[2:]); err == nil {
				host, port := findServer("gobuildslave", *server)

				conn, _ := grpc.Dial(host+":"+strconv.Itoa(port), grpc.WithInsecure())
				defer conn.Close()

				registry := pb.NewBuildSlaveClient(conn)
				res, err := registry.SlaveConfig(context.Background(), &pb.ConfigRequest{})
				if err != nil {
					log.Fatalf("Error listing job: %v", err)
				}
				fmt.Printf("%v\n", res)
			}
		case "config":
			servers := findServers()

			if len(servers) == 0 {
				log.Fatalf("No Servers found!")
			}

			for _, s := range servers {
				conn, _ := grpc.Dial(s.GetIp()+":"+strconv.Itoa(int(s.GetPort())), grpc.WithInsecure())
				defer conn.Close()

				registry := pb.NewGoBuildSlaveClient(conn)
				res, err := registry.GetConfig(context.Background(), &pb.Empty{})
				if err != nil {
					log.Fatalf("Error building job: %v", err)
				}
				fmt.Printf("%v - %v\n", s.GetIdentifier(), res)
			}
		}
	}
}
