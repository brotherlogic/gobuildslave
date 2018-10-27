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

	pbb "github.com/brotherlogic/buildserver/proto"
	pbd "github.com/brotherlogic/discovery/proto"
	pbfc "github.com/brotherlogic/filecopier/proto"
	pbgh "github.com/brotherlogic/githubcard/proto"
	pb "github.com/brotherlogic/gobuildslave/proto"
	"github.com/brotherlogic/goserver"
	pbs "github.com/brotherlogic/goserver/proto"
	"github.com/brotherlogic/goserver/utils"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type discover interface {
	discover(job string, server string) error
}

type prodDiscover struct{}

func (p *prodDiscover) discover(job string, server string) error {
	entries, err := utils.ResolveAll(job)

	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.Identifier == server {
			return nil
		}
	}

	return fmt.Errorf("Unable to find %v on %v", job, server)
}

type prodBuilder struct {
	server func() string
	Log    func(string)
}

func (p *prodBuilder) build(ctx context.Context, job *pb.Job) ([]*pbb.Version, error) {
	ip, port, err := utils.Resolve("buildserver")
	if err != nil {
		return []*pbb.Version{}, err
	}

	conn, err := grpc.Dial(ip+":"+strconv.Itoa(int(port)), grpc.WithInsecure())
	defer conn.Close()
	if err != nil {
		return []*pbb.Version{}, err
	}
	builder := pbb.NewBuildServiceClient(conn)
	versions, err := builder.GetVersions(ctx, &pbb.VersionRequest{Job: job, JustLatest: true})

	if err != nil {
		return []*pbb.Version{}, err
	}

	return versions.Versions, nil
}

func (p *prodBuilder) copy(ctx context.Context, v *pbb.Version) error {
	ip, port, err := utils.Resolve("filecopier")
	if err != nil {
		return err
	}

	conn, err := grpc.Dial(ip+":"+strconv.Itoa(int(port)), grpc.WithInsecure())
	defer conn.Close()
	if err != nil {
		return err
	}
	copier := pbfc.NewFileCopierServiceClient(conn)
	_, err = copier.Copy(ctx, &pbfc.CopyRequest{v.Path, v.Server, "/home/simon/gobuild/bin/" + v.Job.Name, p.server()})
	p.Log(fmt.Sprintf("COPIED %v and %v WITH %v", v.Server, p.server(), err))
	return err
}

type prodDisker struct{}

func (p *prodDisker) getDisks() []string {
	disks := make([]string, 0)
	read, err := ioutil.ReadDir("/media")
	if err == nil {
		for _, f := range read {
			if f.IsDir() {
				disks = append(disks, f.Name())
			}
		}
	}

	return disks
}

// Server the main server type
type Server struct {
	*goserver.GoServer
	runner        *Runner
	disk          diskChecker
	jobs          map[string]*pb.JobDetails
	nMut          *sync.Mutex
	njobs         map[string]*pb.JobAssignment
	translator    translator
	scheduler     *Scheduler
	checker       checker
	disker        disker
	crashFails    int64
	crashError    string
	crashAttempts int64
	builder       Builder
	doesBuild     bool
	discover      discover
	stateMap      map[string]string
	pendingMap    map[time.Weekday]map[string]int
	stateTime     map[string]time.Time
}

func (s *Server) alertOnState(ctx context.Context) {
	for job, t := range s.stateTime {
		if time.Now().Sub(t) > time.Hour {
			s.RaiseIssue(ctx, "Stuck State", fmt.Sprintf("%v is in a stuck state", job), false)
		}
	}
}

func (s *Server) deliverCrashReport(ctx context.Context, j *pb.JobAssignment, output string) {
	s.crashAttempts++
	if len(output) > 0 && !s.SkipLog {
		ip, port := s.GetIP("buildserver")
		s.Log(fmt.Sprintf("GOT BUILDSERVER: %v,%v", ip, port))
		if port > 0 {
			conn, err := grpc.Dial(ip+":"+strconv.Itoa(port), grpc.WithInsecure())
			s.Log(fmt.Sprintf("DIALLED BUILDSERVER: %v", err))
			if err == nil {
				defer conn.Close()
				client := pbb.NewBuildServiceClient(conn)
				_, err := client.ReportCrash(ctx, &pbb.CrashRequest{Job: j.Job, Crash: &pbb.Crash{ErrorMessage: output}})

				s.Log(fmt.Sprintf("REPORTED CRASH: %v", err))
				if err != nil {
					s.crashFails++
					s.crashError = fmt.Sprintf("%v", err)
				}
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
	for job.State != pb.State_DEAD {
		ctx, cancel := utils.BuildContext("nmonitor", job.Job.Name, pbs.ContextType_NO_TRACE)
		defer cancel()
		s.runTransition(ctx, job)
		time.Sleep(time.Second)
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

// updateState of the runner command
func isAlive(ctx context.Context, spec *pb.JobSpec) bool {
	elems := strings.Split(spec.Name, "/")
	if spec.GetPort() == 0 {
		dServer, dPort, err := getIP(ctx, elems[len(elems)-1], spec.Server)

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
	pb.RegisterBuildSlaveServer(server, &s)
}

// ReportHealth determines if the server is healthy
func (s Server) ReportHealth() bool {
	return true
}

// Mote promotes/demotes this server
func (s Server) Mote(ctx context.Context, master bool) error {
	return nil
}

// GetState gets the state of the server
func (s Server) GetState() []*pbs.State {
	return []*pbs.State{
		&pbs.State{Key: "crash_report_fails", Value: s.crashFails},
		&pbs.State{Key: "crash_report_attempts", Value: s.crashAttempts},
		&pbs.State{Key: "crash_reason", Text: s.crashError},
		&pbs.State{Key: "jobs_size", Value: int64(len(s.njobs))},
		&pbs.State{Key: "running_keys", Text: fmt.Sprintf("%v", s.scheduler.rMap)},
		&pbs.State{Key: "trans_state", Text: fmt.Sprintf("%v", s.stateMap)},
		&pbs.State{Key: "pendings", Text: fmt.Sprintf("%v", s.pendingMap)},
	}
}

//Init builds the default runner framework
func Init(b Builder) *Runner {
	r := &Runner{gopath: "goautobuild", m: &sync.Mutex{}, bm: &sync.Mutex{}, builder: b}
	r.runner = runCommand
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

type pTranslator struct{}

func (p *pTranslator) build(job *pb.Job) *exec.Cmd {
	return exec.Command("go", "get", "-u", job.GoPath)
}

func (p *pTranslator) run(job *pb.Job) *exec.Cmd {
	elems := strings.Split(job.GoPath, "/")
	command := elems[len(elems)-1]
	if job.Sudo {
		return exec.Command("sudo", "$GOPATH/bin/"+command)
	}
	return exec.Command("$GOPATH/bin/" + command)
}

type pChecker struct{}

func (p *pChecker) isAlive(ctx context.Context, job *pb.JobAssignment) bool {
	return isJobAlive(ctx, job)
}

// updateState of the runner command
func isJobAlive(ctx context.Context, job *pb.JobAssignment) bool {
	if job.GetPort() == 0 {
		dServer, dPort, err := getIP(ctx, job.Job.Name, job.Server)

		e, ok := status.FromError(err)
		if ok && e.Code() == codes.DeadlineExceeded {
			//Ignore deadline exceeds on discover
			return true
		}

		if err != nil {
			return false
		}

		job.Host = dServer
		job.Port = dPort
	}

	dConn, err := grpc.Dial(job.Host+":"+strconv.Itoa(int(job.Port)), grpc.WithInsecure())
	if err != nil {
		return false
	}
	defer dConn.Close()

	c := pbs.NewGoserverServiceClient(dConn)
	resp, err := c.IsAlive(ctx, &pbs.Alive{}, grpc.FailFast(false))

	if err != nil || resp.Name != job.Job.Name {
		e, ok := status.FromError(err)
		if ok && e.Code() != codes.Unavailable {
			return false
		}
	}

	return true
}

func getIP(ctx context.Context, name string, server string) (string, int32, error) {
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

func (s *Server) checkOnSsh(ctx context.Context) {
	f := "/home/simon/.ssh"

	for true {
		_, err := os.Stat(f)
		if err != nil {
			ip, port, _ := utils.Resolve("githubcard")
			if port > 0 {
				conn, err := grpc.Dial(ip+":"+strconv.Itoa(int(port)), grpc.WithInsecure())
				if err == nil {
					defer conn.Close()
					client := pbgh.NewGithubClient(conn)
					client.AddIssue(ctx, &pbgh.Issue{Service: "gobuildslave", Title: "SSH Needed", Body: s.Registry.Identifier}, grpc.FailFast(false))
				}

			}
		}
		time.Sleep(time.Hour)
	}
}

func (s *Server) checkOnUpdate(ctx context.Context) {
	f := "/var/cache/apt/pkgcache.bin"

	for true {
		info, err := os.Stat(f)
		if err == nil {
			if info.ModTime().Before(time.Now().AddDate(0, -1, 0)) {
				ip, port, _ := utils.Resolve("githubcard")
				if port > 0 {
					conn, err := grpc.Dial(ip+":"+strconv.Itoa(int(port)), grpc.WithInsecure())
					if err == nil {
						defer conn.Close()
						client := pbgh.NewGithubClient(conn)
						client.AddIssue(ctx, &pbgh.Issue{Service: "gobuildslave", Title: "UDPATE NEEDED", Body: fmt.Sprintf("%v -> %v", s.Registry.Identifier, s.Registry.Ip)}, grpc.FailFast(false))
					}
				}

			}
		}
		time.Sleep(time.Hour)
	}
}

func (s *Server) getServerName() string {
	return s.Registry.Identifier
}

func main() {
	var quiet = flag.Bool("quiet", false, "Show all output")
	var build = flag.Bool("builds", true, "Responds to build requests")
	flag.Parse()

	if *quiet {
		log.SetFlags(0)
		log.SetOutput(ioutil.Discard)
	}

	s := Server{&goserver.GoServer{}, Init(&prodBuilder{}), prodDiskChecker{}, make(map[string]*pb.JobDetails), &sync.Mutex{}, make(map[string]*pb.JobAssignment), &pTranslator{}, &Scheduler{cMutex: &sync.Mutex{}, rMutex: &sync.Mutex{}, rMap: make(map[string]*rCommand)}, &pChecker{}, &prodDisker{}, int64(0), "", int64(0), &prodBuilder{}, *build, &prodDiscover{}, make(map[string]string), make(map[time.Weekday]map[string]int), make(map[string]time.Time)}
	s.scheduler = &Scheduler{cMutex: &sync.Mutex{}, rMutex: &sync.Mutex{}, rMap: make(map[string]*rCommand), Log: s.Log}
	s.builder = &prodBuilder{Log: s.Log, server: s.getServerName}
	s.runner.getip = s.GetIP
	s.runner.logger = s.Log
	s.Register = s
	s.PrepServer()
	s.GoServer.Killme = false
	s.RegisterServer("gobuildslave", false)
	s.RegisterServingTask(s.checkOnUpdate)
	s.RegisterServingTask(s.checkOnSsh)
	s.Log(fmt.Sprintf("Running GBS on %v", s.builder))
	err := s.Serve()
	log.Fatalf("Unable to serve: %v", err)
}
