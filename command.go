package main

import (
	"bufio"
	"context"
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brotherlogic/goserver"
	"github.com/brotherlogic/goserver/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	pbb "github.com/brotherlogic/buildserver/proto"
	pbd "github.com/brotherlogic/discovery/proto"
	pbfc "github.com/brotherlogic/filecopier/proto"
	pbgh "github.com/brotherlogic/githubcard/proto"
	pb "github.com/brotherlogic/gobuildslave/proto"
	pbs "github.com/brotherlogic/goserver/proto"
	pbv "github.com/brotherlogic/versionserver/proto"
)

var (
	fails = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gobuildslave_pingfails",
		Help: "The size of the print queue",
	})

	voltage = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gobuildslave_voltage",
		Help: "The size of the print queue",
	})
)

type version interface {
	confirm(ctx context.Context, job string) bool
}

type prodVersion struct {
	dial   func(ctx context.Context, server string) (*grpc.ClientConn, error)
	server string
	log    func(ctx context.Context, line string)
}

func (p *prodVersion) confirm(ctx context.Context, job string) bool {
	conn, err := p.dial(ctx, "versionserver")
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
	p.log(ctx, fmt.Sprintf("Result = %v, %v", err, resp))
	if err != nil {
		return false
	}

	return resp.Success
}

type prodBuilder struct {
	dial   func(ctx context.Context, server string) (*grpc.ClientConn, error)
	server func() string
	Log    func(context.Context, string)
}

func (p *prodBuilder) build(ctx context.Context, job *pb.Job) (*pbb.Version, error) {
	file := fmt.Sprintf("/home/simon/gobuild/bin/%v.version", job.Name)
	data, err := ioutil.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			file := fmt.Sprintf("/home/simon/gobuild/bin/%v.nversion", job.Name)
			data, err = ioutil.ReadFile(file)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	version := &pbb.Version{}
	proto.Unmarshal(data, version)

	return version, nil
}

func (p *prodBuilder) copy(ctx context.Context, v *pbb.Version) (*pbfc.CopyResponse, error) {
	conn, err := p.dial(ctx, "filecopier")
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
	read, err := os.ReadDir("/media")
	if err == nil {
		for _, f := range read {
			if f.IsDir() || f.Name() == "datastore" {
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
	disker          disker
	crashFails      int64
	crashError      string
	crashAttempts   int64
	builder         Builder
	doesBuild       bool
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
	lastAccess      time.Time
	ackChan         chan *pb.JobAssignment
	maxJobs         int
	shuttingDown    bool
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
			blockingQueue:    make(chan *rCommand),
			nonblockingQueue: make(chan *rCommand),
			complete:         make([]*rCommand, 0),
		},
		&prodDisker{},
		int64(0),
		"",
		int64(0),
		&prodBuilder{},
		build,
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
		time.Now(),
		make(chan *pb.JobAssignment, 1000),
		7,
		false,
	}

	// Run the processing queues
	go s.scheduler.processBlockingCommands()
	go s.scheduler.processNonblockingCommands()

	return s
}

func (s *Server) deliverCrashReport(ctx context.Context, j *pb.JobAssignment, output string) {
	s.crashAttempts++

	s.CtxLog(ctx, fmt.Sprintf("Attempting to send crash report: %v -> %v", j.Job, output))

	if j.Job.Name == "buildserver" && len(output) > 0 {
		s.RaiseIssue("Buildserver failing", fmt.Sprintf("on %v -> %v", s.Registry, output))
	}

	if len(strings.TrimSpace(output)) > 0 && !s.SkipLog {
		conn, err := s.FDialServer(ctx, "buildserver")
		if err == nil && s.Registry != nil {
			defer conn.Close()
			client := pbb.NewBuildServiceClient(conn)
			_, err := client.ReportCrash(ctx, &pbb.CrashRequest{Origin: s.Registry.Identifier, Version: j.RunningVersion, Job: j.Job, Crash: &pbb.Crash{ErrorMessage: output}})

			if err != nil {
				s.crashFails++
				s.crashError = fmt.Sprintf("%v-%v", j.Job, err)
				s.CtxLog(ctx, fmt.Sprintf("Failed to send crash report: %v -> %v because %v", j.Job, output, err))

			}
		}
	} else {
		s.CtxLog(ctx, fmt.Sprintf("Skipping empty crash report"))
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
		ctx, cancel := utils.ManualContext(fmt.Sprintf("nmonitor-%v", job.Job.Name), time.Minute)
		s.runTransition(ctx, job)
		cancel()
		time.Sleep(time.Second * 10)
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
	return []*pbs.State{}
}

// Init builds the default runner framework
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
	return exec.Command("go", "install", fmt.Sprintf("%v@latest", job.GoPath))
}

func (p *pTranslator) run(job *pb.Job) *exec.Cmd {
	elems := strings.Split(job.GoPath, "/")
	command := elems[len(elems)-1]
	if job.Sudo {
		return exec.Command("sudo", "$GOPATH/bin/"+command)
	}
	return exec.Command("$GOPATH/bin/" + command)
}

// updateState of the runner command
func (s *Server) isJobAlive(ctx context.Context, job *pb.JobAssignment) bool {
	if job.GetPort() == 0 {
		dServer, dPort, err := s.getIP(ctx, job.Job.Name, job.Server)

		s.CtxLog(ctx, fmt.Sprintf("GOT THE PORT %v with %v", dPort, err))

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

func (s *Server) getIP(ctx context.Context, name string, server string) (string, int32, error) {
	conn, err := s.FDial(utils.LocalDiscover)
	if err != nil {
		return "", -1, err
	}
	defer conn.Close()

	registry := pbd.NewDiscoveryServiceV2Client(conn)
	r, err := registry.Get(ctx, &pbd.GetRequest{Job: name, Server: server})

	if err != nil {
		return "", -1, err
	}
	if len(r.GetServices()) == 0 {
		return "", -1, fmt.Errorf("No services found for %v and %v", name, server)
	}

	s.CtxLog(ctx, fmt.Sprintf("%v -> %v", server, r.GetServices()))

	return r.GetServices()[0].Ip, r.GetServices()[0].Port, nil

}

func (s *Server) checkOnSsh(ctx context.Context) error {
	f := "/home/simon/.ssh"

	for true {
		_, err := os.Stat(f)
		if err != nil {
			conn, err := s.FDialServer(ctx, "githubcard")
			if err == nil {
				defer conn.Close()
				client := pbgh.NewGithubClient(conn)
				client.AddIssue(ctx, &pbgh.Issue{Service: "gobuildslave", Title: "SSH Needed", Body: s.Registry.Identifier}, grpc.FailFast(false))
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
				conn, err := s.FDialServer(ctx, "githubcard")
				if err == nil {
					defer conn.Close()
					client := pbgh.NewGithubClient(conn)
					client.AddIssue(ctx, &pbgh.Issue{Service: "gobuildslave", Title: "UDPATE NEEDED", Body: fmt.Sprintf("%v -> %v", s.Registry.Identifier, s.Registry.Ip)}, grpc.FailFast(false))

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
				s.versions[val.GetJob().GetName()] = val
				s.versionsMutex.Unlock()
			}
		}

	}
}

func (s *Server) getLatestVersion(ctx context.Context, jobName, path string) (*pbb.Version, error) {
	conn, err := s.FDialServer(ctx, "buildserver")
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	client := pbb.NewBuildServiceClient(conn)
	resp, err := client.GetVersions(ctx, &pbb.VersionRequest{BitSize: int32(s.Bits), JustLatest: true, Job: &pb.Job{Name: jobName, GoPath: path}})
	if err != nil {
		return nil, err
	}
	return resp.GetVersions()[0], nil
}

func (s *Server) badHeartChecker(ctx context.Context) error {
	badHearts := s.BadHearts
	if badHearts-s.lastBadHearts > 100 {
		ioutil.WriteFile("/home/simon/gobuildcrash", []byte(fmt.Sprintf("%v bad hearts", badHearts)), 0644)
		//cmd := exec.Command("sudo", "reboot")
		//cmd.Run()
	}
	s.lastBadHearts = badHearts

	return nil
}

func (s *Server) stateChecker(ctx context.Context) error {
	s.nMut.Lock()
	defer s.nMut.Unlock()
	for _, job := range s.njobs {
		if job.State == pb.State_ACKNOWLEDGED && time.Now().Sub(time.Unix(job.LastTransitionTime, 0)) > time.Minute*30 {
			s.CtxLog(ctx, fmt.Sprintf("%v is having a long ACK (%v) on %v", job.Job.Name, time.Now().Sub(time.Unix(job.LastTransitionTime, 0)), s.Registry.Identifier))
		}
	}
	return nil
}

func (s *Server) backgroundRegister() {
	err := fmt.Errorf("Initial error")
	for err != nil {
		err = s.RegisterServerV2(false)
		if err == nil {
			ctx, cancel := utils.ManualContext("gbs-rereg", time.Minute)
			defer cancel()

			conn, err := s.FDial("localhost:50055")
			if err == nil {
				defer conn.Close()
				client := pbd.NewDiscoveryServiceV2Client(conn)
				client.Unregister(ctx, &pbd.UnregisterRequest{Reason: "gbs-restart", Service: &pbd.RegistryEntry{Identifier: s.Registry.Identifier}})
				time.Sleep(time.Second * 5)
				s.RegisterServerV2(false)
			}
		}

		time.Sleep(time.Minute)
	}

	/*if strings.HasPrefix(s.Registry.Identifier, "clust") {
		s.maxJobs = 100
	}*/

	s.version = &prodVersion{s.FDialServer, s.Registry.Identifier, s.CtxLog}
}

func (s *Server) updateAccess() {
	ctx, cancel := utils.ManualContext("gbs-update-access", time.Hour)
	defer cancel()

	for {
		output, err := exec.Command("sudo", "vcgencmd", "measure_volts").CombinedOutput()
		if err != nil {
			s.CtxLog(ctx, fmt.Sprintf("Unable to measure voltage: %v", err))
		} else {
			bits := strings.Split(string(output), "=")
			val, err := strconv.ParseFloat(bits[1][:len(bits[1])-2], 64)
			if err != nil {
				s.CtxLog(ctx, fmt.Sprintf("Unable to pars voltage: %v", err))
			} else {
				voltage.Set(val)
			}
		}

		url := "http://192.168.68.1"
		r, err := http.Get(url)

		if err == nil {
			s.lastAccess = time.Now()
			r.Body.Close()
		} else {
			s.CtxLog(ctx, fmt.Sprintf("Ping fail %v (%v = %v)", err, s.lastAccess, time.Now().Sub(s.lastAccess).Minutes()))
			fails.Inc()
		}

		if time.Now().Sub(s.lastAccess) > time.Minute*5 && time.Now().Hour() < 22 && time.Now().Hour() >= 6 {
			s.CtxLog(ctx, fmt.Sprintf("REBOOTING -> %v, %v\n", err, s.lastAccess))
			cmd := exec.Command("sudo", "reboot")
			cmd.Run()
		}

		foundIP := false
		ifaces, err := net.Interfaces()
		if err != nil {
			s.CtxLog(ctx, fmt.Sprintf("NETINT ERROR: %v", err))
			foundIP = true
		} else {
			for _, i := range ifaces {
				addrs, err := i.Addrs()
				if err != nil {
					s.CtxLog(ctx, fmt.Sprintf("ADDR ERROR: %v", err))
					foundIP = true
				} else {
					for _, addr := range addrs {
						var ip net.IP
						switch v := addr.(type) {
						case *net.IPNet:
							ip = v.IP
						case *net.IPAddr:
							ip = v.IP
						}
						// process IP address

						if ip != nil && !ip.IsLoopback() && ip.String() != "127.0.0.1" {
							foundIP = true
							s.CtxLog(ctx, fmt.Sprintf("FOUNDIP %v", ip))
						}
					}
				}
			}
		}
		if foundIP {
			s.CtxLog(ctx, fmt.Sprintf("NOIP"))
		}

		time.Sleep(time.Second * 30)
	}
}

func (s *Server) lookForDiscover(ctx context.Context) error {
	for _, job := range s.njobs {
		if job.GetJob().GetName() == "discovery" {
			if job.GetState() == pb.State_RUNNING {
				return nil
			}
			if time.Now().Sub(time.Unix(job.GetLastTransitionTime(), 0)) > time.Minute*10 {
				s.RaiseIssue("Discover is in a bad state", fmt.Sprintf("[%v] %v is the current state at %v (trans at %v)", s.Registry, job, time.Now(), time.Unix(job.GetLastTransitionTime(), 0)))
			}
			return nil
		}
	}

	s.RaiseIssue("Missing discover", fmt.Sprintf("Discover is missing on %v (%v)", s.Registry, s.njobs))
	return nil
}

func (s *Server) sleepDisplay() {
	for !s.LameDuck {
		command := []string{"-display", ":0.0", "dpms", "force", "on"}
		if time.Now().Hour() >= 22 || time.Now().Hour() < 7 {
			command = []string{"-display", ":0.0", "dpms", "force", "off"}
		}
		exec.Command("xset", command...).Run()

		time.Sleep(time.Minute * 5)
	}
}

func main() {
	var quiet = flag.Bool("quiet", false, "Show all output")
	var build = flag.Bool("builds", true, "Responds to build requests")
	var maxnum = flag.Int("max_jobs", 10, "Maxiumum jobs")
	flag.Parse()

	if *quiet {
		log.SetFlags(0)
		log.SetOutput(ioutil.Discard)
	}

	s := InitServer(*build)

	s.scheduler.Log = s.CtxLog
	s.builder = &prodBuilder{Log: s.CtxLog, server: s.getServerName, dial: s.FDialServer}
	s.runner.getip = s.GetIP
	s.runner.logger = s.CtxLog
	s.Register = s
	s.PrepServer("gobuildslave")
	s.Killme = false

	s.maxJobs = *maxnum
	dets, err := ioutil.ReadFile("/sys/firmware/devicetree/base/model")
	if err == nil {
		model := string(dets)
		if strings.HasPrefix(model, "Raspberry Pi 4") {
			s.maxJobs = 100
		} else {
			s.maxJobs = 0
		}

	}

	go s.backgroundRegister()

	s.loadCurrentVersions()

	// Run a discover server to allow us to do a local register
	ctx, cancel := utils.ManualContext("gbs", time.Minute)
	defer cancel()
	_, err = s.RunJob(ctx, &pb.RunRequest{Job: &pb.Job{
		Name:             "discovery",
		GoPath:           "github.com/brotherlogic/discovery",
		PartialBootstrap: true,
		Breakout:         true,
	}})
	if err != nil {
		log.Fatalf("Error in setup:%v", err)
	}
	// Run a gobuildmaster to get jobs running
	_, err = s.RunJob(ctx, &pb.RunRequest{Job: &pb.Job{
		Name:             "gobuildmaster",
		GoPath:           "github.com/brotherlogic/gobuildmaster",
		PartialBootstrap: true,
		Breakout:         true,
	}})
	if err != nil {
		log.Fatalf("Error in setup: %v", err)
	}

	// Wait until we can register
	for s.Registry == nil {
		time.Sleep(time.Minute)
	}

	go s.procAcks()
	go s.updateAccess()
	if strings.Contains(s.Registry.Identifier, "display") || strings.Contains(s.Registry.Identifier, "personal") {
		go s.sleepDisplay()
	}

	err = s.Serve()
	log.Fatalf("Unable to serve: %v", err)
}
