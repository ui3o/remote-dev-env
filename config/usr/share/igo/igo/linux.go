//go:build !mac

package main

import (
	"syscall"
	"time"
)

func zombieInit() {
	go func() {
		for {
			// Wait for any child process (WNOHANG: don't block if none)
			var ws syscall.WaitStatus
			var ru syscall.Rusage
			for {
				pid, err := syscall.Wait4(-1, &ws, syscall.WNOHANG, &ru)
				if pid <= 0 || err != nil {
					break
				}
			}
			time.Sleep(1 * time.Second)
		}
	}()
}
