package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"

	"google.golang.org/grpc"

	"github.com/brotherlogic/goserver"
)

// Server the main server type
type Server struct {
	*goserver.GoServer
	runner *Runner
}

// DoRegister Registers this server
func (s Server) DoRegister(server *grpc.Server) {
	//Nothing to register at the moment
}

//Init builds the default runner framework
func Init() *Runner {
	r := &Runner{gopath: "goautobuild"}
	r.runner = runCommand
	go r.run()
	return r
}

func runCommand(c *runnerCommand) {
	log.Printf("RUNNING COMMAND")
	env := os.Environ()
	home := ""
	for _, s := range env {
		if strings.HasPrefix(s, "HOME=") {
			home = s[5:]
		}
	}

	if len(home) == 0 {

	}

	env = append(env, fmt.Sprintf("GOPATH="+home+"gobuild"))
	c.command.Env = env

	out, err := c.command.StdoutPipe()
	if err != nil {

	}

	c.command.Start()

	buf := new(bytes.Buffer)
	buf.ReadFrom(out)
	str := buf.String()

	c.command.Wait()

	c.output = str
	c.complete = true
}

func main() {
	s := Server{&goserver.GoServer{}, Init()}
	s.Register = s
	s.PrepServer()
	s.RegisterServer("gobuildslave", false)
	s.Serve()
}
