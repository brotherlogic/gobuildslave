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

func findServer(ctx context.Context, name, server string) (string, int) {
	conn, _ := grpc.Dial(utils.Discover, grpc.WithInsecure())
	defer conn.Close()

	registry := pbdi.NewDiscoveryServiceClient(conn)
	rs, _ := registry.ListAllServices(ctx, &pbdi.ListRequest{})

	for _, r := range rs.GetServices().Services {
		if r.Identifier == server && r.Name == name {
			return r.Ip, int(r.Port)
		}
	}

	return "", -1
}

func findServers(ctx context.Context) []*pbdi.RegistryEntry {
	conn, _ := grpc.Dial(utils.Discover, grpc.WithInsecure())
	defer conn.Close()

	registry := pbdi.NewDiscoveryServiceClient(conn)
	rs, _ := registry.ListAllServices(ctx, &pbdi.ListRequest{})

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
	var hosta = buildFlags.String("hosta", "", "Name of the server to build on")

	ctx, cancel := utils.BuildContext("gobuildslavecli-"+os.Args[1], "gobuildslave")
	defer cancel()

	if len(os.Args) <= 1 {
		fmt.Printf("Commands: build run\n")
	} else {
		switch os.Args[1] {
		case "build":
			if err := buildFlags.Parse(os.Args[2:]); err == nil {
				host, port := findServer(ctx, "gobuildslave", *server)

				conn, _ := grpc.Dial(host+":"+strconv.Itoa(port), grpc.WithInsecure())
				defer conn.Close()

				registry := pb.NewGoBuildSlaveClient(conn)
				_, err := registry.BuildJob(ctx, &pb.JobSpec{Name: *name})
				if err != nil {
					log.Fatalf("Error building job: %v", err)
				}
			}
		case "run":
			if err := buildFlags.Parse(os.Args[2:]); err == nil {
				host, port := findServer(ctx, "gobuildslave", *server)

				conn, _ := grpc.Dial(host+":"+strconv.Itoa(port), grpc.WithInsecure())
				defer conn.Close()

				registry := pb.NewGoBuildSlaveClient(conn)
				_, err := registry.Run(ctx, &pb.JobSpec{Name: *name, Server: *server})
				if err != nil {
					log.Fatalf("Error building job: %v", err)
				}
			}
		case "kill":
			if err := buildFlags.Parse(os.Args[2:]); err == nil {
				host, port := findServer(ctx, "gobuildslave", *server)

				conn, _ := grpc.Dial(host+":"+strconv.Itoa(port), grpc.WithInsecure())
				defer conn.Close()

				registry := pb.NewGoBuildSlaveClient(conn)
				_, err := registry.Kill(ctx, &pb.JobSpec{Name: *name, Server: *server})
				if err != nil {
					log.Fatalf("Error building job: %v", err)
				}
			}
		case "list":
			if err := buildFlags.Parse(os.Args[2:]); err == nil {
				host, port := findServer(ctx, "gobuildslave", *server)

				conn, _ := grpc.Dial(host+":"+strconv.Itoa(port), grpc.WithInsecure())
				defer conn.Close()

				registry := pb.NewGoBuildSlaveClient(conn)
				res, err := registry.List(ctx, &pb.Empty{})
				if err != nil {
					log.Fatalf("Error building job: %v", err)
				}
				for _, r := range res.Details {
					fmt.Printf("%v (%v) - %v\n", r.Spec.Name, time.Unix(r.StartTime, 0).Format("02/01 15:04"), r)
				}
			}
		case "nlist":
			if err := buildFlags.Parse(os.Args[2:]); err == nil {
				conn, err := grpc.Dial(*hosta+":53604", grpc.WithInsecure())
				if err != nil {
					log.Fatalf("Error dialling: %v", err)
				}
				defer conn.Close()

				registry := pb.NewBuildSlaveClient(conn)
				res, err := registry.ListJobs(ctx, &pb.ListRequest{})
				if err != nil {
					log.Fatalf("Error listing job with: %v", err)
				}
				for _, r := range res.Jobs {
					fmt.Printf("%v -> %v [%v] (%v)\n", r.Job.Name, r.State, r.GetSubState(), time.Unix(r.GetLastUpdateTime(), 0))
				}
			}
		case "nbuild":
			if err := buildFlags.Parse(os.Args[2:]); err == nil {
				host, port := findServer(ctx, "gobuildslave", *server)

				conn, _ := grpc.Dial(host+":"+strconv.Itoa(port), grpc.WithInsecure())
				defer conn.Close()

				registry := pb.NewBuildSlaveClient(conn)
				_, err := registry.RunJob(ctx, &pb.RunRequest{Job: &pb.Job{Name: "recordalerting", GoPath: "github.com/brotherlogic/recordalerting"}})
				if err != nil {
					log.Fatalf("Error listing job: %v", err)
				}
			}
		case "nconfig":
			if err := buildFlags.Parse(os.Args[2:]); err == nil {
				host, port := findServer(ctx, "gobuildslave", *server)

				conn, _ := grpc.Dial(host+":"+strconv.Itoa(port), grpc.WithInsecure())
				defer conn.Close()

				registry := pb.NewBuildSlaveClient(conn)
				res, err := registry.SlaveConfig(ctx, &pb.ConfigRequest{})
				if err != nil {
					log.Fatalf("Error listing job: %v", err)
				}
				fmt.Printf("%v\n", res)
			}
		case "config":
			servers := findServers(ctx)

			if len(servers) == 0 {
				log.Fatalf("No Servers found!")
			}

			for _, s := range servers {
				conn, _ := grpc.Dial(s.GetIp()+":"+strconv.Itoa(int(s.GetPort())), grpc.WithInsecure())
				defer conn.Close()

				registry := pb.NewBuildSlaveClient(conn)
				res, err := registry.SlaveConfig(ctx, &pb.ConfigRequest{})
				if err != nil {
					log.Fatalf("Error listing job: %v", err)
				}
				fmt.Printf("%v. %v\n", s.Identifier, res)
			}
		}

	}
}
