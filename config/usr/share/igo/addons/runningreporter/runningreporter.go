package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"go.senan.xyz/flagconf"
)

type RuntimeConfig struct {
	Interval int
}

var (
	Config = RuntimeConfig{}
)

func init() {
	flag.CommandLine.Init("env_param_runningreporter", flag.ExitOnError)

	flag.IntVar(&Config.Interval, "interval", 150, "default is 150ms")

	flag.Parse()
	flagconf.ParseEnv()

	confJson, err := json.MarshalIndent(Config, "", "  ")
	if err != nil {
		log.Println("Failed to marshal config to JSON:", err)
	} else {
		log.Println("RuntimeConfig JSON:", string(confJson))
	}

}

func reportRunning() {
	username := os.Getenv("DEVELOPER")
	if username == "" {
		username = "unknown"
	}
	dirPath := filepath.Join("/tmp/.runtime/logins", username)
	filePath := filepath.Join(dirPath, ".running")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if err := os.MkdirAll(dirPath, 0755); err == nil {
			f, err := os.Create(filePath)
			if err == nil {
				log.Println("Created file to:", filePath)
				f.Close()
				cmd := exec.Command("cp", "-rf", "/run/secrets/runtime/.", dirPath)
				if out, err := cmd.Output(); err != nil {
					log.Println("cp has error:", err)
					return
				} else {
					log.Println("cp out:", string(out))
				}
			} else {
				log.Println("Error creating file:", err)
			}
		} else {
			log.Println("Error creating directory:", err)
		}
	}
}

func main() {
	go func() {
		for {
			reportRunning()
			time.Sleep(time.Duration(Config.Interval) * time.Millisecond)
		}
	}()
	select {}
}
