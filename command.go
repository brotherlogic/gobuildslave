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
	"sync"
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
			job.StartTime = 0
			job.State = pb.JobDetails_BUILDING
			s.runner.Checkout(job.GetSpec().Name)
			job.State = pb.JobDetails_BUILT
		case pb.JobDetails_BUILT:
			s.runner.Run(job)
			for job.StartTime == 0 {
				time.Sleep(waitTime)
			}
			job.State = pb.JobDetails_PENDING
		case pb.JobDetails_KILLING:
			s.runner.kill(job)
			if !isAlive(job.GetSpec()) {
				job.State = pb.JobDetails_DEAD
			}
		case pb.JobDetails_UPDATE_STARTING:
			s.runner.Update(job)
			job.State = pb.JobDetails_RUNNING
		case pb.JobDetails_PENDING:
			time.Sleep(time.Minute)
			if isAlive(job.GetSpec()) {
				job.State = pb.JobDetails_RUNNING
			} else {
				job.State = pb.JobDetails_DEAD
			}
		case pb.JobDetails_RUNNING:
			time.Sleep(waitTime)
			if !isAlive(job.GetSpec()) {
				job.State = pb.JobDetails_DEAD
			}
		case pb.JobDetails_DEAD:
			log.Printf("RERUNNING BECAUSE WERE DEAD")
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
	r, err := registry.Discover(context.Background(), &entry)

	if err != nil {
		return "", -1
	}

	return r.Ip, int(r.Port)
}

// updateState of the runner command
func isAlive(spec *pb.JobSpec) bool {
	elems := strings.Split(spec.Name, "/")
	dServer, dPort := getIP(elems[len(elems)-1], spec.Server)

	if dPort > 0 {
		dConn, err := grpc.Dial(dServer+":"+strconv.Itoa(dPort), grpc.WithInsecure())
		if err != nil {
			return false
		}
		defer dConn.Close()

		c := pbs.NewGoserverServiceClient(dConn)
		_, err = c.IsAlive(context.Background(), &pbs.Alive{})

		if err != nil {
			log.Printf("FOUND DEAD SERVER: %v", err)
		}

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
	r := &Runner{gopath: "goautobuild", m: &sync.Mutex{}}
	r.runner = runCommand
	go r.run()
	return r
}

func runCommand(c *runnerCommand) {
	if c == nil || c.command == nil {
		return
	}

	env := os.Environ()
	home := ""
	for _, s := range env {
		if strings.HasPrefix(s, "HOME=") {
			home = s[5:]
		}
	}

	gpath := home + "/gobuild"
	c.command.Path = strings.Replace(c.command.Path, "$GOPATH", gpath, -1)
	for i := range c.command.Args {
		c.command.Args[i] = strings.Replace(c.command.Args[i], "$GOPATH", gpath, -1)
	}

	path := fmt.Sprintf("GOPATH=" + home + "/gobuild")
	found := false
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
	c.command.Env = envl

	out, _ := c.command.StdoutPipe()

	c.command.Start()

	if !c.background {
		str := ""

		if out != nil {
			buf := new(bytes.Buffer)
			buf.ReadFrom(out)
			str = buf.String()
		}

		c.command.Wait()
		c.output = str
		c.complete = true
	} else {
		c.details.StartTime = time.Now().Unix()
	}
}

func (diskChecker prodDiskChecker) diskUsage(path string) int64 {
	return diskUsage(path)
}

func (s *Server) rebuildLoop() {
	for true {
		time.Sleep(time.Minute * 60)

		var rebuildList []*pb.JobDetails
		var hashList []string
		for _, job := range s.runner.backgroundTasks {
			if time.Since(job.started) > time.Hour {
				rebuildList = append(rebuildList, job.details)
				hashList = append(hashList, job.hash)
			}
		}

		for i := range rebuildList {
			s.runner.Rebuild(rebuildList[i], hashList[i])
		}
	}
}

func main() {
	var quiet = flag.Bool("quiet", false, "Show all output")
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
