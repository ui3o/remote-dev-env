package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
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

	flag.IntVar(&Config.Interval, "interval", 1, "default is 1s")

	flag.Parse()
	flagconf.ParseEnv()

	confJson, err := json.MarshalIndent(Config, "", "  ")
	if err != nil {
		log.Println("Failed to marshal config to JSON:", err)
	} else {
		log.Println("RuntimeConfig JSON:", string(confJson))
	}

}

func main() {
	go func() {
		for {
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
					} else {
						log.Println("Error creating file:", err)
					}
				} else {
					log.Println("Error creating directory:", err)
				}
			}
			time.Sleep(time.Duration(Config.Interval) * time.Second)
		}
	}()
	select {}
}
