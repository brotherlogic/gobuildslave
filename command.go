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
	jobs   map[string]*pb.JobDetails
}

func (s *Server) monitor(job *pb.JobDetails) {
	for true {
		switch job.State {
		case pb.JobDetails_ACKNOWLEDGED:
			job.State = pb.JobDetails_BUILDING
			s.runner.Checkout(job.GetSpec().Name)
			job.State = pb.JobDetails_BUILT
		case pb.JobDetails_BUILT:
			s.runner.Run(job.GetSpec())
			job.State = pb.JobDetails_PENDING
		case pb.JobDetails_KILLING:
			s.runner.kill(job.GetSpec())
			if !isAlive(job.GetSpec()) {
				job.State = pb.JobDetails_DEAD
			}
		case pb.JobDetails_UPDATE_STARTING:
			s.runner.Update(job.GetSpec())
			job.State = pb.JobDetails_RUNNING
		case pb.JobDetails_PENDING:
			time.Sleep(waitTime)
			if isAlive(job.GetSpec()) {
				job.State = pb.JobDetails_RUNNING
			}
		case pb.JobDetails_RUNNING:
			time.Sleep(waitTime)
			if !isAlive(job.GetSpec()) {
				job.State = pb.JobDetails_DEAD
			}
		case pb.JobDetails_DEAD:
			job.State = pb.JobDetails_ACKNOWLEDGED
		}
	}
}

func getHash(file string) (string, error) {
	env := os.Environ()
	home := ""
	for _, s := range env {
		if strings.HasPrefix(s, "HOME=") {
			home = s[5:]
		}
	}

	if len(home) == 0 {
		log.Printf("Error in home: %v", home)
	}
	gpath := home + "/gobuild"

	f, err := os.Open(strings.Replace(file, "$GOPATH", gpath, 1))
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
	log.Printf("Searching for %v", entry)
	r, err := registry.Discover(context.Background(), &entry)

	if err != nil {
		log.Printf("Lookup failed for %v,%v -> %v", name, server, err)
		return "", -1
	}

	return r.Ip, int(r.Port)
}

// updateState of the runner command
func isAlive(spec *pb.JobSpec) bool {
	elems := strings.Split(spec.Name, "/")
	dServer, dPort := getIP(elems[len(elems)-1], spec.Server)

	log.Printf("Unable to find: %v -> %v", elems, dPort)
	if dPort > 0 {
		dConn, err := grpc.Dial(dServer+":"+strconv.Itoa(dPort), grpc.WithInsecure())
		if err != nil {
			return false
		}
		defer dConn.Close()

		c := pbs.NewGoserverServiceClient(dConn)
		_, err = c.IsAlive(context.Background(), &pbs.Alive{})
		log.Printf("UPDATED: %v,%v -> %v", dServer, dPort, err)
		return err == nil
	}

	//Mark as false if we can't locate the job
	return false
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

	if c.command == nil {
		return
	}

	env := os.Environ()
	home := ""
	for _, s := range env {
		if strings.HasPrefix(s, "HOME=") {
			home = s[5:]
		}
	}

	if len(home) == 0 {
		log.Printf("Error in home: %v", home)
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

	log.Printf("RUNNING %v, %v and %v", c.command.Path, c.command.Args, c.command.Env)
	c.command.Start()
	log.Printf("RUN STARTED:  %v", c.background)

	if !c.background {
		str := ""

		if out != nil {
			buf := new(bytes.Buffer)
			log.Printf("RUN READING 1:%v", out)
			buf.ReadFrom(out)
			str = buf.String()
			log.Printf("RUN IS HERE: %v", str)
		}

		if out2 != nil {
			buf2 := new(bytes.Buffer)
			log.Printf("RUN READING 2")
			buf2.ReadFrom(out2)
			str2 := buf2.String()
			log.Printf("RUN NOW %v and %v", str, str2)

		}
		log.Print("RUN HAS STARTING TO WAIT")
		c.command.Wait()
		log.Printf("RUN DONE WAITING")
		c.output = str
		c.complete = true
	}
	log.Printf("RUN IS DONE")
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

	s := Server{&goserver.GoServer{}, Init(), prodDiskChecker{}, make(map[string]*pb.JobDetails)}
	s.Register = s
	s.PrepServer()
	s.GoServer.Killme = false
	s.RegisterServingTask(s.rebuildLoop)
	s.RegisterServer("gobuildslave", false)
	s.Serve()
}
