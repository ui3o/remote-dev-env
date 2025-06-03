//go:build !mac

package main

import (
	"github.com/ramr/go-reaper"
)

func zombieInit() {
	go reaper.Reap()
}
