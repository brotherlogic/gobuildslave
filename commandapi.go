package main

import (
	"bytes"
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/brotherlogic/goserver"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pbd "github.com/brotherlogic/discovery/proto"
	pb "github.com/brotherlogic/gobuildslave/proto"
	pbs "github.com/brotherlogic/goserver/proto"
)

// Server the main server type
type Server struct {
	*goserver.GoServer
	runner *Runner
	disk   diskChecker
}

func getHash(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return string(h.Sum(nil)), nil
}

func getIP(name string, server string) (string, int) {
	conn, _ := grpc.Dial("192.168.86.64:50055", grpc.WithInsecure())
	defer conn.Close()

	registry := pbd.NewDiscoveryServiceClient(conn)
	entry := pbd.RegistryEntry{Name: name, Identifier: server}
	r, err := registry.Discover(context.Background(), &entry)

	if err != nil {
		log.Printf("Lookup failed for %v,%v -> %v", name, server, err)
		return "", -1
	}

	return r.Ip, int(r.Port)
}

// updateState of the runner command
func updateState(com *runnerCommand) {
	elems := strings.Split(com.details.Spec.Name, "/")
	dServer, dPort := getIP(elems[len(elems)-1], com.details.Spec.Server)

	log.Printf("Unable to find: %v -> %v", elems, dPort)
	if dPort > 0 {
		dConn, err := grpc.Dial(dServer+":"+strconv.Itoa(dPort), grpc.WithInsecure())
		if err != nil {
			panic(err)
		}
		defer dConn.Close()

		c := pbs.NewGoserverServiceClient(dConn)
		_, err = c.IsAlive(context.Background(), &pbs.Alive{})
		log.Printf("UPDATED: %v,%v -> %v", dServer, dPort, err)
		com.details.Running = (err == nil)
	} else {
		//Mark as false if we can't locate the job
		com.details.Running = false
	}
}

// BuildJob builds out a job
func (s *Server) BuildJob(ctx context.Context, in *pb.JobSpec) (*pb.Empty, error) {
	s.runner.Checkout(in.Name)
	return &pb.Empty{}, nil
}

// List lists all running jobs
func (s *Server) List(ctx context.Context, in *pb.Empty) (*pb.JobList, error) {
	details := &pb.JobList{}
	for _, job := range s.runner.backgroundTasks {
		updateState(job)
		details.Details = append(details.Details, job.details)
	}

	return details, nil
}

// Run runs a background task
func (s *Server) Run(ctx context.Context, in *pb.JobSpec) (*pb.Empty, error) {
	s.runner.Run(in)
	return &pb.Empty{}, nil
}

//Update restarts a job with new settings
func (s *Server) Update(ctx context.Context, in *pb.JobSpec) (*pb.Empty, error) {
	s.runner.Update(in)
	return &pb.Empty{}, nil
}

// Kill a background task
func (s *Server) Kill(ctx context.Context, in *pb.JobSpec) (*pb.Empty, error) {
	s.runner.kill(in)
	return &pb.Empty{}, nil
}

// DoRegister Registers this server
func (s Server) DoRegister(server *grpc.Server) {
	pb.RegisterGoBuildSlaveServer(server, &s)
}

// ReportHealth determines if the server is healthy
func (s Server) ReportHealth() bool {
	return true
}

// Mote promotes/demotes this server
func (s Server) Mote(master bool) error {
	return nil
}

//Init builds the default runner framework
func Init() *Runner {
	r := &Runner{gopath: "goautobuild"}
	r.runner = runCommand
	go r.run()
	return r
}

func runCommand(c *runnerCommand) {
	log.Printf("RUNNING COMMAND: %v", c)
	env := os.Environ()
	home := ""
	for _, s := range env {
		if strings.HasPrefix(s, "HOME=") {
			home = s[5:]
		}
	}

	if len(home) == 0 {

	}
	gpath := home + "/gobuild"
	c.command.Path = strings.Replace(c.command.Path, "$GOPATH", gpath, -1)
	for i := range c.command.Args {
		c.command.Args[i] = strings.Replace(c.command.Args[i], "$GOPATH", gpath, -1)
	}

	path := fmt.Sprintf("GOPATH=" + home + "/gobuild")
	found := false
	log.Printf("HERE = %v", c.command.Env)
	envl := os.Environ()
	for i, blah := range envl {
		if strings.HasPrefix(blah, "GOPATH") {
			envl[i] = path
			found = true
		}
	}
	if !found {
		envl = append(envl, path)
	}
	log.Printf("ENV = %v", envl)
	c.command.Env = envl

	out, err := c.command.StdoutPipe()
	out2, err2 := c.command.StderrPipe()
	if err != nil {
		log.Printf("Blah: %v", err)
	}

	if err2 != nil {
		log.Printf("Blah2: %v", err)
	}

	log.Printf("%v, %v and %v", c.command.Path, c.command.Args, c.command.Env)
	c.command.Start()

	if !c.background {
		buf := new(bytes.Buffer)
		buf.ReadFrom(out)
		str := buf.String()

		buf2 := new(bytes.Buffer)
		buf2.ReadFrom(out2)
		str2 := buf2.String()
		log.Printf("%v and %v", str, str2)

		c.command.Wait()
		c.output = str
		c.complete = true
	}
	log.Printf("DONE")
}

func (diskChecker prodDiskChecker) diskUsage(path string) int64 {
	return diskUsage(path)
}

func (s *Server) rebuildLoop() {
	for true {
		time.Sleep(time.Minute)

		var rebuildList []*pb.JobSpec
		var hashList []string
		for _, job := range s.runner.backgroundTasks {
			log.Printf("Job (started %v, now %v) %v", job.started, time.Now(), job)
			if time.Since(job.started) > time.Hour {
				log.Printf("Added to rebuild list (%v)", job)
				rebuildList = append(rebuildList, job.details.Spec)
				hashList = append(hashList, job.hash)
			}
		}

		log.Printf("Rebuilding %v", rebuildList)
		for i := range rebuildList {
			s.runner.Rebuild(rebuildList[i], hashList[i])
		}
	}
}

func main() {
	var quiet = flag.Bool("quiet", true, "Show all output")
	flag.Parse()

	if *quiet {
		log.SetFlags(0)
		log.SetOutput(ioutil.Discard)
	}

	s := Server{&goserver.GoServer{}, Init(), prodDiskChecker{}}
	s.Register = s
	s.PrepServer()
	s.RegisterServingTask(s.rebuildLoop)
	s.RegisterServer("gobuildslave", false)
	s.Serve()
}
