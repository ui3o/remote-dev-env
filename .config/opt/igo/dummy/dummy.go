package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	message := flag.String("message", "Dummy origin is running", "Message to display on startup")
	sleepDuration := flag.Int("sleep", -1, "Sleep duration in seconds (use a negative number for infinite sleep)")
	defaultExitCode := flag.Int("exitcode", 1, "The exit code this program will return when done")

	flag.Parse()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println(*message)
	fmt.Println("Params: ", os.Args[1:])

	if *sleepDuration < 0 {
		fmt.Println("Sleeping indefinitely. Press Ctrl+C to shutdown gracefully")
		<-sigs
	} else {
		select {
		case <-time.After(time.Duration(*sleepDuration) * time.Second):
		case <-sigs:
		}
	}

	fmt.Println("Received termination signal. Exiting...")
	os.Exit(*defaultExitCode)
}
