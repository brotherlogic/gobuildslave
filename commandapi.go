package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

//Init builds the default runner framework
func Init() *Runner {
	r := &Runner{gopath: "goautobuild"}
	r.runner = runCommand
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
	r := Init()
	go r.run()
	log.Printf("STARTING SLEEP")
	time.Sleep(10000 * time.Millisecond)
	log.Printf("DONE MAIN")
}
