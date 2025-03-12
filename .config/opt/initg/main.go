package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
)

func main() {

	fmt.Println("tail:", flag.Args())
	flag.Parse()

	go func() {
		cmd := exec.Command(flag.Args()[0], flag.Args()[1])
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	}()

	go func() {
		cmd := exec.Command("sleep", "5555")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	}()
	cmd := exec.Command("sleep", "inf")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	// c := make(chan os.Signal, 2)
	// signal.Notify(c, os.Interrupt, os.Kill)
	// go func() {
	// 	<-c
	// 	// cleanup
	// 	cmd.Process.Kill()
	// 	os.Exit(1)
	// }()
}
