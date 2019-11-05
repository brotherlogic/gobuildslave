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
	"runtime"
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
	pbv "github.com/brotherlogic/versionserver/proto"
	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type version interface {
	confirm(ctx context.Context, job string) bool
}

type prodVersion struct {
	dial   func(server string) (*grpc.ClientConn, error)
	server string
	log    func(line string)
}

func (p *prodVersion) confirm(ctx context.Context, job string) bool {
	conn, err := p.dial("versionserver")
	if err != nil {
		return false
	}
	defer conn.Close()

	client := pbv.NewVersionServerClient(conn)

	setTime := time.Now().Add(time.Minute).Unix()
	resp, err := client.SetIfLessThan(ctx,
		&pbv.SetIfLessThanRequest{
			TriggerValue: time.Now().Unix(),
			Set: &pbv.Version{
				Key:    "guard." + job,
				Value:  setTime,
				Setter: "gobuildslave" + p.server,
			},
		})
	p.log(fmt.Sprintf("Result = %v, %v", err, resp))
	if err != nil {
		return false
	}

	return resp.Success
}

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
	dial   func(server string) (*grpc.ClientConn, error)
	server func() string
	Log    func(string)
}

func (p *prodBuilder) build(ctx context.Context, job *pb.Job) (*pbb.Version, error) {
	file := fmt.Sprintf("/home/simon/gobuild/bin/%v.version", job.Name)
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	version := &pbb.Version{}
	proto.Unmarshal(data, version)

	return version, nil
}

func (p *prodBuilder) copy(ctx context.Context, v *pbb.Version) (*pbfc.CopyResponse, error) {
	conn, err := p.dial("filecopier")
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	copier := pbfc.NewFileCopierServiceClient(conn)
	req := &pbfc.CopyRequest{
		InputFile:    v.Path,
		InputServer:  v.Server,
		OutputFile:   "/home/simon/gobuild/bin/" + v.Job.Name,
		OutputServer: p.server(),
	}
	return copier.QueueCopy(ctx, req)
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
	runner          *Runner
	disk            diskChecker
	jobs            map[string]*pb.JobDetails
	nMut            *sync.Mutex
	njobs           map[string]*pb.JobAssignment
	translator      translator
	scheduler       *Scheduler
	checker         checker
	disker          disker
	crashFails      int64
	crashError      string
	crashAttempts   int64
	builder         Builder
	doesBuild       bool
	discover        discover
	stateMap        map[string]string
	pendingMap      map[time.Weekday]map[string]int
	stateMutex      *sync.Mutex
	rejecting       bool
	lastCopyTime    time.Duration
	lastCopyStatus  string
	versionsMutex   *sync.Mutex
	versions        map[string]*pbb.Version
	skippedCopies   int64
	copies          int64
	version         version
	lastBadHearts   int
	accessPoint     string
	discoverStartup time.Time
	discoverSync    time.Time
}

// InitServer builds out a server
func InitServer(build bool) *Server {
	s := &Server{
		&goserver.GoServer{},
		Init(&prodBuilder{}),
		prodDiskChecker{},
		make(map[string]*pb.JobDetails),
		&sync.Mutex{},
		make(map[string]*pb.JobAssignment),
		&pTranslator{},
		&Scheduler{
			cMutex: &sync.Mutex{},
			rMutex: &sync.Mutex{},
			rMap:   make(map[string]*rCommand),
		},
		&pChecker{},
		&prodDisker{},
		int64(0),
		"",
		int64(0),
		&prodBuilder{},
		build,
		&prodDiscover{},
		make(map[string]string),
		make(map[time.Weekday]map[string]int),
		&sync.Mutex{},
		build,
		0,
		"",
		&sync.Mutex{},
		make(map[string]*pbb.Version),
		int64(0),
		int64(0),
		&prodVersion{},
		0,
		"",
		time.Now(),
		time.Now(),
	}
	return s
}

func (s *Server) deliverCrashReport(ctx context.Context, j *pb.JobAssignment, output string) {
	s.crashAttempts++

	if j.Job.Name == "buildserver" {
		s.RaiseIssue(ctx, "Buildserver failing", fmt.Sprintf("%v", output), false)
	}

	s.Log(fmt.Sprintf("Sending %v for %v", output, j))

	if len(output) > 0 && !s.SkipLog {
		conn, err := s.DialMaster("buildserver")
		if err == nil {
			defer conn.Close()
			client := pbb.NewBuildServiceClient(conn)
			_, err := client.ReportCrash(ctx, &pbb.CrashRequest{Version: j.RunningVersion, Job: j.Job, Crash: &pbb.Crash{ErrorMessage: output}})

			if err != nil {
				s.crashFails++
				s.crashError = fmt.Sprintf("%v-%v", j.Job, err)
			}
		}
	} else {
		s.Log(fmt.Sprintf("Skipping empty crash report"))
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
		ctx, cancel := utils.BuildContext("nmonitor", job.Job.Name)
		s.runTransition(ctx, job)
		cancel()
		time.Sleep(time.Second * 30)
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
func (s *Server) DoRegister(server *grpc.Server) {
	pb.RegisterGoBuildSlaveServer(server, s)
	pb.RegisterBuildSlaveServer(server, s)
}

// ReportHealth determines if the server is healthy
func (s *Server) ReportHealth() bool {
	return true
}

func (s *Server) Shutdown(ctx context.Context) error {
	return nil
}

// Mote promotes/demotes this server
func (s *Server) Mote(ctx context.Context, master bool) error {
	return nil
}

// GetState gets the state of the server
func (s *Server) GetState() []*pbs.State {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()

	oldest := time.Now().Unix()
	stale := int64(0)
	for _, cm := range s.scheduler.rMap {
		if cm.startTime < oldest {
			oldest = cm.startTime
		}
		if cm.endTime > 0 {
			stale++
		}
	}

	s.versionsMutex.Lock()
	defer s.versionsMutex.Unlock()
	return []*pbs.State{
		&pbs.State{Key: "discover_start", TimeValue: s.discoverStartup.Unix()},
		&pbs.State{Key: "discover_sync", TimeValue: s.discoverSync.Unix()},
		&pbs.State{Key: "state_map", Text: fmt.Sprintf("%v", s.stateMap)},
		&pbs.State{Key: "access_point", Text: s.accessPoint},
		&pbs.State{Key: "oldest_command", TimeValue: oldest},
		&pbs.State{Key: "stale_commands", Value: stale},
		&pbs.State{Key: "crash_report_fails", Value: s.crashFails},
		&pbs.State{Key: "crash_report_attempts", Value: s.crashAttempts},
		&pbs.State{Key: "crash_reason", Text: s.crashError},
		&pbs.State{Key: "jobs_size", Value: int64(len(s.njobs))},
		&pbs.State{Key: "running_keys", Value: int64(len(s.scheduler.rMap))},
		&pbs.State{Key: "go_version", Text: fmt.Sprintf("%v", runtime.Version())},
		&pbs.State{Key: "reject", Text: fmt.Sprintf("%v", s.rejecting)},
		&pbs.State{Key: "last_copy_time", TimeDuration: s.lastCopyTime.Nanoseconds()},
		&pbs.State{Key: "last_copy_status", Text: s.lastCopyStatus},
		&pbs.State{Key: "versions", Value: int64(len(s.versions))},
		&pbs.State{Key: "copies", Value: s.copies},
		&pbs.State{Key: "skipped_copies", Value: s.skippedCopies},
		&pbs.State{Key: "commands", Value: int64(len(s.scheduler.rMap))},
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

func (s *Server) checkOnSsh(ctx context.Context) error {
	f := "/home/simon/.ssh"

	for true {
		_, err := os.Stat(f)
		if err != nil {
			ip, port, _ := utils.Resolve("githubcard", "gobuildslave-checkonssh")
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

	return nil
}

func (s *Server) checkOnUpdate(ctx context.Context) error {
	f := "/var/cache/apt/pkgcache.bin"

	for true {
		info, err := os.Stat(f)
		if err == nil {
			if info.ModTime().Before(time.Now().AddDate(0, -1, 0)) {
				ip, port, _ := utils.Resolve("githubcard", "gobuildslave-checkonupdate")
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

	return nil
}

func (s *Server) getServerName() string {
	return s.Registry.Identifier
}

func (s *Server) loadCurrentVersions() {
	files, err := ioutil.ReadDir("/home/simon/gobuild/bin")
	if err == nil {
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".version") {
				data, _ := ioutil.ReadFile("/home/simon/gobuild/bin/" + f.Name())
				val := &pbb.Version{}
				proto.Unmarshal(data, val)
				s.versionsMutex.Lock()
				s.versions[val.Job.Name] = val
				s.versionsMutex.Unlock()
			}
		}

	}
}

func (s *Server) cleanCommands(ctx context.Context) error {
	for !s.LameDuck {
		time.Sleep(time.Minute)
		s.scheduler.clean()
	}

	return nil
}

func (s *Server) badHeartChecker(ctx context.Context) error {
	badHearts := s.BadHearts
	if badHearts-s.lastBadHearts > 100 {
		ioutil.WriteFile("/home/simon/gobuildcrash", []byte(fmt.Sprintf("%v bad hearts", badHearts)), 0644)
		cmd := exec.Command("sudo", "reboot")
		cmd.Run()
	}
	s.lastBadHearts = badHearts

	return nil
}

func (s *Server) stateChecker(ctx context.Context) error {
	s.nMut.Lock()
	defer s.nMut.Unlock()
	for _, job := range s.njobs {
		if job.State == pb.State_ACKNOWLEDGED && time.Now().Sub(time.Unix(job.LastTransitionTime, 0)) > time.Minute*30 {
			s.Log(fmt.Sprintf("%v is having a long ACK (%v) on %v", job.Job.Name, time.Now().Sub(time.Unix(job.LastTransitionTime, 0)), s.Registry.Identifier))
		}
	}
	return nil
}

func main() {
	var quiet = flag.Bool("quiet", false, "Show all output")
	var build = flag.Bool("builds", true, "Responds to build requests")
	flag.Parse()

	if *quiet {
		log.SetFlags(0)
		log.SetOutput(ioutil.Discard)
	}

	s := InitServer(*build)
	s.scheduler = &Scheduler{cMutex: &sync.Mutex{}, rMutex: &sync.Mutex{}, rMap: make(map[string]*rCommand), Log: s.Log}
	s.builder = &prodBuilder{Log: s.Log, server: s.getServerName, dial: s.DialMaster}
	s.runner.getip = s.GetIP
	s.runner.logger = s.Log
	s.Register = s
	s.PrepServer()
	s.GoServer.Killme = false
	err := s.RegisterServer("gobuildslave", false)
	if err != nil {
		log.Fatalf("Error registering: %v", err)
	}

	err = s.unregisterChildren()
	if err != nil {
		log.Fatalf("Error unregistering: %v", err)
	}

	s.version = &prodVersion{s.DialMaster, s.Registry.Identifier, s.Log}

	s.RegisterServingTask(s.checkOnUpdate, "check_on_update")
	s.RegisterServingTask(s.checkOnSsh, "check_on_ssh")
	s.RegisterServingTask(s.cleanCommands, "clean_commands")
	s.RegisterRepeatingTaskNonMaster(s.trackUpTime, "track_up_time", time.Hour)
	s.RegisterRepeatingTask(s.stateChecker, "state_checker", time.Minute*5)
	s.RegisterRepeatingTask(s.badHeartChecker, "bad_heart_checker", time.Minute*5)

	s.loadCurrentVersions()

	err = s.Serve()
	log.Fatalf("Unable to serve: %v", err)
}
