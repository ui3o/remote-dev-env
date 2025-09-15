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
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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
	Idle LockFileType = iota
	Lock
	Hold
	Block
)

type PortDefinition struct {
	FullName     string
	Name         string
	Port         string
	Opened       bool
	LocalServer  *http.Server
	GlobalServer *http.Server
}

type PortSet map[string]PortDefinition

func (pd PortDefinition) Construct(port, name string) {
	Config.GlobalPortList[port] = PortDefinition{Name: name, Port: port, FullName: name + "." + port}
	touchFile(port, Idle)
}

func (pd PortDefinition) SetServer(LocalServer, GlobalServer *http.Server) {
	pd.LocalServer = LocalServer
	pd.GlobalServer = GlobalServer
	Config.GlobalPortList[pd.Port] = pd
}
func (pd PortDefinition) SetOpened(opened bool) {
	pd.Opened = opened
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

func checkPortOpened() {
	cmd := exec.Command("lsof", "-i", "-P")
	if output, err := cmd.Output(); err != nil {
		return
	} else {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, " (LISTEN)") {
				command := strings.Split(line, " ")[0]
				port := strings.Split(strings.Split(line, " (LISTEN)")[0], ":")[1]
				if _, exists := Config.GlobalPortList[port]; exists {
					Config.GlobalPortList[port].SetOpened(true)
					if strings.HasPrefix(command, "portlocke") {
						touchFile(port, Hold)
					} else {
						touchFile(port, Lock)
					}
				}
			}
		}
	}
}

func init() {
	flag.CommandLine.Init("env_param_portlocker_addon", flag.ExitOnError)

	portSet := make(PortSet)
	flag.Var(&portSet, "global_port_list", "GRAFANA,PROMETHEUS,LOKI,...")
	Config.Developer = os.Getenv("DEVELOPER")

	// Remove the lock directory recursively before starting main loop
	lockDir := "/tmp/.runtime/global_ports/" + Config.Developer + "/"
	if err := os.RemoveAll(lockDir); err != nil {
		log.Fatalf("Failed to remove lock directory recursively: %v", err)
	}
	// Create the lock directory
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		log.Fatalf("Failed to create lock directory: %v", err)
	}

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
			touchFile(port, Block)
			return
		}
		globalListen, err := net.Listen("tcp", ":"+port)
		if err != nil {
			// log.Printf("Failed to start global server: %v", err)
			Config.GlobalPortList[port].ShutdownServer()
			touchFile(port, Block)
			return
		}

		go Config.GlobalPortList[port].LocalServer.Serve(localListen)
		go Config.GlobalPortList[port].GlobalServer.Serve(globalListen)
		removeBlockFiles(port)
		log.Println("Creating server for:", Config.GlobalPortList[port].Name, "on port:", Config.GlobalPortList[port].Port)
	}
}

func findLockFiles() {
	lockDir := "/tmp/.runtime/global_ports/"
	lockPortList := make(PortSet)
	if err := filepath.Walk(lockDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && !strings.Contains(path, Config.Developer) &&
			(strings.HasSuffix(info.Name(), ".lock") || strings.HasSuffix(info.Name(), ".block")) {
			f := strings.TrimSuffix(info.Name(), ".lock")
			f = strings.TrimSuffix(f, ".block")
			p := strings.Split(f, ".")[1]
			name := strings.Split(f, ".")[0]

			if _, exists := Config.GlobalPortList[p]; !exists {
				log.Println("Port", p, "not found in GlobalPortList, have to add new server definition.")
				Config.GlobalPortList[p].Construct(p, name)
			}
			if strings.HasSuffix(info.Name(), ".lock") {
				lockPortList[p] = Config.GlobalPortList[p]
				createServer(p)
			}
		}
		return nil
	}); err != nil {
		log.Printf("Error during hold file scanning, the path %q: %v\n", lockDir, err)
	}
	if err := filepath.Walk(lockDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.Contains(path, Config.Developer) && strings.HasSuffix(info.Name(), ".block") {
			f := strings.TrimSuffix(info.Name(), ".block")
			p := strings.Split(f, ".")[1]
			if _, exists := lockPortList[p]; !exists {
				removeBlockFiles(p)
				if Config.GlobalPortList[p].Opened {
					touchFile(p, Lock)
				} else {
					touchFile(p, Idle)
				}
			}
		}
		if !info.IsDir() && strings.Contains(path, Config.Developer) && strings.HasSuffix(info.Name(), ".lock") {
			f := strings.TrimSuffix(info.Name(), ".lock")
			p := strings.Split(f, ".")[1]
			if !Config.GlobalPortList[p].Opened {
				touchFile(p, Idle)
			}

		}
		if !info.IsDir() && strings.Contains(path, Config.Developer) && strings.HasSuffix(info.Name(), ".hold") {
			f := strings.TrimSuffix(info.Name(), ".hold")
			p := strings.Split(f, ".")[1]
			if _, exists := lockPortList[p]; !exists {
				Config.GlobalPortList[p].ShutdownServer()
				touchFile(p, Idle)
			}
		}
		return nil
	}); err != nil {
		log.Printf("Error walking the path %q: %v\n", lockDir, err)
	}
}

func createFile(touchFile, ext string, valid []string, removeOnCreate []string) {
	isValid := false
	for _, v := range valid {
		if _, err := os.Stat(touchFile + "." + v); err == nil {
			isValid = true
		}
	}
	if !isValid {
		if _, err := os.Stat(touchFile + "." + ext); err != nil {
			f, err := os.Create(touchFile + "." + ext)
			if err != nil {
				log.Printf("Failed to create file: %v", err)
			} else {
				log.Printf("Create file: %v", touchFile+"."+ext)
				f.Close()
			}
		}

		for _, v := range removeOnCreate {
			if _, err := os.Stat(touchFile + "." + v); err == nil {
				if err := os.Remove(touchFile + "." + v); err != nil {
					log.Printf("Failed to remove lock file %s: %v", touchFile+"."+v, err)
				}
			}
		}
	}
}

func removeBlockFiles(port string) {
	userDir := "/tmp/.runtime/global_ports/" + Config.Developer + "/"
	touchFile := userDir + Config.GlobalPortList[port].FullName + ".block"
	if _, err := os.Stat(touchFile); err == nil {
		if err := os.Remove(touchFile); err != nil {
			log.Printf("Failed to remove block file %s: %v", touchFile, err)
		}
	}
}

func touchFile(port string, fileType LockFileType) {
	userDir := "/tmp/.runtime/global_ports/" + Config.Developer + "/"
	touchFile := userDir + Config.GlobalPortList[port].FullName
	switch fileType {
	case Lock:
		createFile(touchFile, "lock", []string{"block"}, []string{"idle"})
	case Hold:
		createFile(touchFile, "hold", []string{}, []string{"idle"})
	case Block:
		createFile(touchFile, "block", []string{}, []string{"idle", "lock"})
	default:
		createFile(touchFile, "idle", []string{}, []string{"block", "lock", "hold"})
	}
}

func main() {
	for {
		for port := range Config.GlobalPortList {
			Config.GlobalPortList[port].SetOpened(false)
		}
		checkPortOpened()
		findLockFiles()
		time.Sleep(1 * time.Second)
	}
}
