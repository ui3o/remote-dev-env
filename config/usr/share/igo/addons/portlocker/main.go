package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dario.cat/mergo"
	"go.senan.xyz/flagconf"
)

type RuntimeConfig struct {
	GlobalPortList PortSet
	Developer      string
}

var (
	Config = RuntimeConfig{
		GlobalPortList: make(PortSet),
	}
)

type LockFileType int

const (
	Lock LockFileType = iota
	Hold
	Block
)

type PortDefinition struct {
	FullName      string
	Name          string
	Port          string
	NativeOpened  bool
	VirtualOpened bool
	LocalServer   *http.Server
	GlobalServer  *http.Server
}

type PortSet map[string]PortDefinition

func (pd PortDefinition) Construct(port, name string) {
	Config.GlobalPortList[port] = PortDefinition{Name: name, Port: port, FullName: name + "." + port}
}

func (pd PortDefinition) SetNativeOpened(opened bool) {
	pd.NativeOpened = opened
	Config.GlobalPortList[pd.Port] = pd
}

func (pd PortDefinition) SetServer(LocalServer, GlobalServer *http.Server) {
	pd.LocalServer = LocalServer
	pd.GlobalServer = GlobalServer
	Config.GlobalPortList[pd.Port] = pd
}

func (pd PortDefinition) SetVirtualOpened(virtual bool) {
	pd.VirtualOpened = virtual
	Config.GlobalPortList[pd.Port] = pd
}

func (pd PortDefinition) ShutdownServer() {
	if pd.LocalServer != nil {
		pd.LocalServer.Shutdown(context.Background())
		pd.LocalServer = nil
	}
	if pd.GlobalServer != nil {
		pd.GlobalServer.Shutdown(context.Background())
		pd.GlobalServer = nil
	}
	Config.GlobalPortList[pd.Port] = pd
}

func (ps *PortSet) String() string {
	var ports []string
	for name, config := range *ps {
		ports = append(ports, fmt.Sprintf("%s:%s", name, config.Name))
	}
	return strings.Join(ports, ",")
}

func getPortName(port string) (string, string) {
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "PORT_") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 && parts[1] == port {
				return strings.Split(parts[0], "_")[1], port
			}
		}
	}
	return port, port
}

func (ps *PortSet) Set(value string) error {
	portEnv := "PORT_" + strings.ToUpper(value)
	port := os.Getenv(portEnv)
	if port == "" {
		return fmt.Errorf("environment variable %s not set", portEnv)
	}
	n, p := getPortName(port)
	Config.GlobalPortList[port].Construct(p, n)
	return nil
}

func isPortOpened(port string) bool {
	timeout := 50 * time.Millisecond
	if local, err := net.DialTimeout("tcp", "127.0.0.1:"+port, timeout); err != nil {
		if global, err := net.DialTimeout("tcp", "0.0.0.0:"+port, timeout); err != nil {
			return false
		} else {
			defer global.Close()
			return true
		}
	} else {
		defer local.Close()
		return true
	}
}

func init() {
	flag.CommandLine.Init("env_param_portlocker_addon", flag.ExitOnError)

	portSet := make(PortSet)
	flag.Var(&portSet, "global_port_list", "GRAFANA,PROMETHEUS,LOKI,...")
	Config.Developer = os.Getenv("DEVELOPER")

	flag.Parse()
	flagconf.ParseEnv()

	confJson, err := json.MarshalIndent(Config, "", "  ")
	if err != nil {
		log.Println("[INIT] Failed to marshal config to JSON:", err)
	} else {
		log.Println("[INIT] RuntimeConfig JSON:", string(confJson))
	}
}

func createServer(port string) {
	Config.GlobalPortList[port].SetVirtualOpened(true)
	if Config.GlobalPortList[port].LocalServer == nil {
		Config.GlobalPortList[port].SetServer(
			&http.Server{
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(599)
				}),
			}, &http.Server{
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(599)
				}),
			})

		localListen, err := net.Listen("tcp", "localhost:"+port)
		if err != nil {
			// log.Printf("Failed to start local server: %v", err)
			Config.GlobalPortList[port].ShutdownServer()
			return
		}
		globalListen, err := net.Listen("tcp", ":"+port)
		if err != nil {
			// log.Printf("Failed to start global server: %v", err)
			Config.GlobalPortList[port].ShutdownServer()
			return
		}

		go Config.GlobalPortList[port].LocalServer.Serve(localListen)
		go Config.GlobalPortList[port].GlobalServer.Serve(globalListen)
		log.Println("Creating server for:", Config.GlobalPortList[port].Name, "on port:", Config.GlobalPortList[port].Port)
	}
}

// func findLockFiles(lockList []string, listeningSet PortListeningSet, blockedSet BlockedSet) {
func findLockFiles() {
	lockDir := "/tmp/.runtime/global_ports/"
	err := filepath.Walk(lockDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".lock") && !strings.Contains(path, Config.Developer) {
			f := strings.TrimSuffix(info.Name(), ".lock")
			p := strings.Split(f, ".")[1]
			name := strings.Split(f, ".")[0]

			if _, exists := Config.GlobalPortList[p]; !exists {
				log.Println("Port", p, "not found in GlobalPortList, have to add new server definition.")
				Config.GlobalPortList[p].Construct(p, name)
			}
			createServer(p)
		}
		return nil
	})
	if err != nil {
		log.Printf("Error walking the path %q: %v\n", lockDir, err)
	}
}

func removeLockFiles(port string) {
	lockDir := "/tmp/.runtime/global_ports/" + Config.Developer + "/"
	err := filepath.Walk(lockDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.Contains(info.Name(), "."+port+".") {
			if err := os.Remove(path); err != nil {
				log.Printf("Failed to remove lock file %s: %v", path, err)
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("Error walking the path %q: %v\n", lockDir, err)
	}
}

func touchFile(port string, fileType LockFileType) {
	userDir := "/tmp/.runtime/global_ports/" + Config.Developer + "/"

	touchedFile := userDir + Config.GlobalPortList[port].FullName
	switch fileType {
	case Lock:
		touchedFile = touchedFile + ".lock"
	case Hold:
		touchedFile = touchedFile + ".hold"
	default:
		touchedFile = touchedFile + ".block"
	}

	if _, err := os.Stat(touchedFile); err != nil {
		log.Println("Create file:", touchedFile)
		f, err := os.Create(touchedFile)
		if err != nil {
			log.Printf("Failed to create file: %v", err)
		} else {
			f.Close()
		}
	}
}

func main() {
	// Remove the lock directory recursively before starting main loop
	lockDir := "/tmp/.runtime/global_ports/" + Config.Developer + "/"
	if err := os.RemoveAll(lockDir); err != nil {
		log.Fatalf("Failed to remove lock directory recursively: %v", err)
	}
	// Create the lock directory
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		log.Fatalf("Failed to create lock directory: %v", err)
	}

	for {
		tmpPortList := make(PortSet)
		mergo.Merge(&tmpPortList, Config.GlobalPortList)
		for port, config := range Config.GlobalPortList {
			if status := isPortOpened(port); status {
				if !config.VirtualOpened {
					Config.GlobalPortList[port].SetNativeOpened(true)
				}
			} else {
				if config.NativeOpened {
					Config.GlobalPortList[port].SetNativeOpened(false)
					removeLockFiles(port)
				}
			}
			if config.VirtualOpened {
				Config.GlobalPortList[port].SetVirtualOpened(false)
			}
		}

		findLockFiles()

		for port, config := range Config.GlobalPortList {
			if !config.VirtualOpened && config.LocalServer != nil {
				// Stop the local server if it's running
				log.Printf("Have to stop internal servers on: %v", port)
				Config.GlobalPortList[port].ShutdownServer()
				removeLockFiles(port)
			}
			if !config.VirtualOpened && tmpPortList[port].VirtualOpened && config.NativeOpened {
				removeLockFiles(port)
			}
			if config.VirtualOpened && config.LocalServer != nil {
				touchFile(port, Hold)
			}
			if config.NativeOpened {
				touchFile(port, Lock)
			}
			if config.NativeOpened && config.VirtualOpened {
				touchFile(port, Block)
			}
		}
		time.Sleep(1 * time.Second)
	}
}
