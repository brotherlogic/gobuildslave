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
	runner *Runner
	disk   diskChecker
	jobs   map[string]*pb.JobDetails
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
				job.TestCount++
			} else {
				job.TestCount = 0
			}
			if job.TestCount > 3 {
				s.Log(fmt.Sprintf("Killing beacuse we couldn't reach 3 times: %v", job))
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := grpc.Dial(utils.RegistryIP+":"+strconv.Itoa(utils.RegistryPort), grpc.WithInsecure())
	if err == nil {
		defer conn.Close()

		registry := pbd.NewDiscoveryServiceClient(conn)
		entry := pbd.RegistryEntry{Name: name, Identifier: server}
		r, err2 := registry.Discover(ctx, &entry, grpc.FailFast(false))

		if err2 == nil {
			return r.Ip, int(r.Port)
		}
	}
	return "", -1
}

// updateState of the runner command
func isAlive(spec *pb.JobSpec) bool {
	elems := strings.Split(spec.Name, "/")
	dServer, dPort := getIP(elems[len(elems)-1], spec.Server)

	if dPort > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		dConn, err := grpc.Dial(dServer+":"+strconv.Itoa(dPort), grpc.WithInsecure())
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

func main() {
	var quiet = flag.Bool("quiet", false, "Show all output")
	flag.Parse()

	if *quiet {
		log.SetFlags(0)
		log.SetOutput(ioutil.Discard)
	}

	s := Server{&goserver.GoServer{}, Init(), prodDiskChecker{}, make(map[string]*pb.JobDetails)}
	s.runner.getip = s.GetIP
	s.runner.logger = s.Log
	s.Register = s
	s.PrepServer()
	s.GoServer.Killme = false
	s.RegisterServingTask(s.rebuildLoop)
	s.RegisterServer("gobuildslave", false)
	s.Serve()
}
