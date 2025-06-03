//go:build !mac

package main

import (
	"fmt"

	"github.com/ramr/go-reaper"
)

func zombieInit() {
	fmt.Println("hi")
	go reaper.Reap()
}
