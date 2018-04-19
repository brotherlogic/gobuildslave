package main

import (
	"bufio"
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brotherlogic/goserver"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbd "github.com/brotherlogic/discovery/proto"
	pbgh "github.com/brotherlogic/githubcard/proto"
	pb "github.com/brotherlogic/gobuildslave/proto"
	pbs "github.com/brotherlogic/goserver/proto"
	"github.com/brotherlogic/goserver/utils"
)

// Server the main server type
type Server struct {
	*goserver.GoServer
	runner     *Runner
	disk       diskChecker
	jobs       map[string]*pb.JobDetails
	njobs      map[string]*pb.JobAssignment
	translator translator
	scheduler  *Scheduler
}

func deliverCrashReport(job *runnerCommand, getter func(name string) (string, int), logger func(text string)) {
	ip, port := getter("githubcard")
	if port > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		conn, err := grpc.Dial(ip+":"+strconv.Itoa(port), grpc.WithInsecure())
		if err == nil {
			defer conn.Close()
			client := pbgh.NewGithubClient(conn)
			elems := strings.Split(job.details.Spec.GetName(), "/")
			if len(job.output) > 0 {
				client.AddIssue(ctx, &pbgh.Issue{Service: elems[len(elems)-1], Title: "CRASH REPORT", Body: job.output}, grpc.FailFast(false))
			}
		}
	}
}

func (s *Server) addMessage(details *pb.JobDetails, message string) {
	for _, t := range s.runner.backgroundTasks {
		if t.details.GetSpec().Name == details.Spec.Name {
			t.output += message
		}
	}
}

func (s *Server) nmonitor(job *pb.JobAssignment) {
	// Do nothing for now
}

func (s *Server) monitor(job *pb.JobDetails) {
	for true {
		switch job.State {
		case pb.State_ACKNOWLEDGED:
			job.StartTime = 0
			job.GetSpec().Port = 0
			job.State = pb.State_BUILDING
			s.runner.Checkout(job.GetSpec().Name)
			job.State = pb.State_BUILT
		case pb.State_BUILT:
			s.runner.Run(job)
			for job.StartTime == 0 {
				time.Sleep(waitTime)
			}
			job.State = pb.State_PENDING
		case pb.State_KILLING:
			s.runner.kill(job)
			if !isAlive(job.GetSpec()) {
				job.State = pb.State_DEAD
			}
		case pb.State_UPDATE_STARTING:
			s.runner.Update(job)
			job.State = pb.State_UPDATE_STARTING
		case pb.State_PENDING:
			time.Sleep(time.Minute)
			if isAlive(job.GetSpec()) {
				job.State = pb.State_RUNNING
			} else {
				job.State = pb.State_DEAD
			}
		case pb.State_RUNNING:
			time.Sleep(waitTime)
			if !isAlive(job.GetSpec()) {
				job.TestCount++
			} else {
				job.TestCount = 0
			}
			if job.TestCount > 60 {
				s.Log(fmt.Sprintf("Killing beacuse we couldn't reach 60 times: %v", job))
				job.State = pb.State_DEAD
			}
		case pb.State_DEAD:
			job.State = pb.State_ACKNOWLEDGED
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

func getIP(name string, server string) (string, int32, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	conn, err := grpc.Dial(utils.RegistryIP+":"+strconv.Itoa(utils.RegistryPort), grpc.WithInsecure())
	defer conn.Close()

	if err != nil {
		return "", -1, err
	}

	registry := pbd.NewDiscoveryServiceClient(conn)
	entry := pbd.RegistryEntry{Name: name, Identifier: server}
	r, err := registry.Discover(ctx, &pbd.DiscoverRequest{Request: &entry}, grpc.FailFast(false))

	if err != nil {
		return "", -1, err
	}

	return r.GetService().Ip, r.GetService().Port, nil
}

// updateState of the runner command
func isAlive(spec *pb.JobSpec) bool {
	elems := strings.Split(spec.Name, "/")
	if spec.GetPort() == 0 {
		dServer, dPort, err := getIP(elems[len(elems)-1], spec.Server)

		e, ok := status.FromError(err)
		if ok && e.Code() == codes.DeadlineExceeded {
			//Ignore deadline exceeds on discover
			return true
		}

		if err != nil {
			return false
		}

		spec.Host = dServer
		spec.Port = dPort
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	dConn, err := grpc.Dial(spec.Host+":"+strconv.Itoa(int(spec.Port)), grpc.WithInsecure())
	if err != nil {
		return false
	}
	defer dConn.Close()

	c := pbs.NewGoserverServiceClient(dConn)
	resp, err := c.IsAlive(ctx, &pbs.Alive{}, grpc.FailFast(false))

	if err != nil || resp.Name != elems[len(elems)-1] {
		e, ok := status.FromError(err)
		if ok && e.Code() != codes.Unavailable {
			return false
		}
	}

	return true
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

// GetState gets the state of the server
func (s Server) GetState() []*pbs.State {
	return []*pbs.State{}
}

//Init builds the default runner framework
func Init() *Runner {
	r := &Runner{gopath: "goautobuild", m: &sync.Mutex{}, bm: &sync.Mutex{}}
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

	path := fmt.Sprintf("GOPATH=" + home + "/gobuild/")
	pathbin := fmt.Sprintf("GOBIN=" + home + "/gobuild/bin/")
	found := false
	envl := os.Environ()
	for i, blah := range envl {
		if strings.HasPrefix(blah, "GOPATH") {
			envl[i] = path
			found = true
		}
		if strings.HasPrefix(blah, "GOBIN") {
			envl[i] = pathbin
			found = true
		}
	}
	if !found {
		envl = append(envl, path)
	}
	c.command.Env = envl

	out, err := c.command.StderrPipe()
	if err != nil {
		log.Fatalf("Problem getting stderr: %v", err)
	}

	if out != nil {
		scanner := bufio.NewScanner(out)
		go func() {
			for scanner != nil && scanner.Scan() {
				c.output += scanner.Text()
			}
			out.Close()
		}()
	}

	c.command.Start()

	if !c.background {
		c.command.Wait()
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

type pTranslator struct{}

func (p *pTranslator) build(job *pb.Job) *exec.Cmd {
	return exec.Command("go", "get", job.Name)
}

func main() {
	var quiet = flag.Bool("quiet", false, "Show all output")
	flag.Parse()

	if *quiet {
		log.SetFlags(0)
		log.SetOutput(ioutil.Discard)
	}

	s := Server{&goserver.GoServer{}, Init(), prodDiskChecker{}, make(map[string]*pb.JobDetails), make(map[string]*pb.JobAssignment), &pTranslator{}, &Scheduler{cMutex: &sync.Mutex{}}}
	s.runner.getip = s.GetIP
	s.runner.logger = s.Log
	s.Register = s
	s.PrepServer()
	s.GoServer.Killme = false
	s.RegisterServingTask(s.rebuildLoop)
	s.RegisterServer("gobuildslave", false)
	err := s.Serve()
	log.Fatalf("Unable to serve: %v", err)
}
