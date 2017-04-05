package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/brotherlogic/goserver"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pb "github.com/brotherlogic/gobuildslave/proto"
)

// Server the main server type
type Server struct {
	*goserver.GoServer
	runner *Runner
}

// BuildJob builds out a job
func (s *Server) BuildJob(ctx context.Context, in *pb.JobSpec) (*pb.Empty, error) {
	s.runner.Checkout(in.Name)
	return &pb.Empty{}, nil
}

// Run runs a background task
func (s *Server) Run(ctx context.Context, in *pb.JobSpec) (*pb.Empty, error) {
	s.runner.Run(in.Name)
	return &pb.Empty{}, nil
}

// DoRegister Registers this server
func (s Server) DoRegister(server *grpc.Server) {
	pb.RegisterGoBuildSlaveServer(server, &s)
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

	buf := new(bytes.Buffer)
	buf.ReadFrom(out)
	str := buf.String()

	buf2 := new(bytes.Buffer)
	buf2.ReadFrom(out2)
	str2 := buf2.String()
	log.Printf("%v and %v", str, str2)

	if !c.background {
		c.command.Wait()
		c.output = str
		c.complete = true
	}
}

func main() {
	s := Server{&goserver.GoServer{}, Init()}
	s.Register = s
	s.PrepServer()
	s.RegisterServer("gobuildslave", false)
	s.Serve()
}
