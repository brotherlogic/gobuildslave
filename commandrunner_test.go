package main

import "log"
import "testing"
import "time"

func InitTest() *Runner {
	r := &Runner{}
	r.runner = testRunCommand

	go r.run()

	return r
}

func testRunCommand(c *runnerCommand) {
	//We do nothing
	log.Printf("WHAT")
	time.Sleep(200 * time.Millisecond)
	log.Printf("SLEPT")
	c.complete = true
}

func TestRun(t *testing.T) {
	r := InitTest()
	r.Run("testrepo")
	r.LameDuck(true)
	if r.commandsRun != 1 {
		t.Errorf("Not enough commands: (%v) %v", r.commandsRun, r.commands)
	}
}

func TestCheckout(t *testing.T) {
	r := InitTest()
	log.Printf("TESTREPO CHECKOUT")
	r.Checkout("testrepo")
	log.Printf("LAMEDUCKING")
	r.LameDuck(true)

	if r.commandsRun != 2 {
		t.Errorf("Not enough commands: %v", r.commands)
	}
}
