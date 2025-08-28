package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.senan.xyz/flagconf"
)

type RuntimeConfig struct {
	GlobalPortList              PortSet
	MaxRetryCountForPortOpening int
	Developer                   string
}

var (
	Config = RuntimeConfig{
		GlobalPortList: make(PortSet),
	}
)

type PortDefinition struct {
	Internal bool
	Name     string
}

type ServerDefinition struct {
	Running bool
	Server  *http.Server
}

type PortSet map[string]PortDefinition
type PortListeningSet map[string]ServerDefinition

func (ps *PortSet) String() string {
	var ports []string
	for name, config := range *ps {
		ports = append(ports, fmt.Sprintf("%s:%s", name, config.Name))
	}
	return strings.Join(ports, ",")
}

func getPortName(port string) string {
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "PORT_") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 && parts[1] == port {
				return strings.Split(parts[0], "_")[1] + "." + port
			}
		}
	}
	return port
}

func (ps *PortSet) Set(value string) error {
	portEnv := "PORT_" + strings.ToUpper(value)
	port := os.Getenv(portEnv)
	if port == "" {
		return fmt.Errorf("environment variable %s not set", portEnv)
	}
	Config.GlobalPortList[port] = PortDefinition{
		Internal: false,
		Name:     getPortName(port),
	}
	return nil
}

func debugHeader(username string) string {
	return fmt.Sprintf("[%s] ", username)
}

func checkPortIsOpened(port string) error {
	for i := Config.MaxRetryCountForPortOpening; i > 0; i-- {
		cmd := exec.Command("nc", "-z", "localhost", port)
		err := cmd.Run()
		if err == nil {
			// log.Println(debugHeader(Config.Developer), "Port is opened: ", port)
			return nil
		} else {
			// log.Println(debugHeader(Config.Developer), "Port is not available yet for:", port)
			time.Sleep(100 * time.Microsecond)
		}
	}
	return errors.New("port is not available after retries")
}

func init() {
	flag.CommandLine.Init("env_param_portlocker_addon", flag.ExitOnError)
	flag.IntVar(&Config.MaxRetryCountForPortOpening, "max_retry_count_for_port_opening", 5, "default is 5 so 5x100ms=500ms")

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

func createServer(port string) *http.Server {
	srv := &http.Server{
		Addr: ":" + port,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(599)
		}),
	}
	go func() {
		srv.ListenAndServe()
		// srv.Shutdown(context.Background())
	}()
	return srv
}

func findLockedFiles(lockList []string, portSet PortListeningSet) {
	lockDir := "/tmp/.runtime/global_ports/"
	err := filepath.Walk(lockDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".lock") && !strings.Contains(path, Config.Developer) {
			f := strings.TrimSuffix(info.Name(), ".lock")
			p := strings.Split(f, ".")[1]
			// log.Println("Found lock file:", path)
			canCreate := true
			for _, lockedPort := range lockList {
				if strings.Split(lockedPort, ".")[1] == p {
					canCreate = false
				}
			}
			if canCreate {
				if _, exists := Config.GlobalPortList[p]; !exists {
					log.Println("Port", p, "not found in GlobalPortList, skipping server creation.")
					Config.GlobalPortList[p] = PortDefinition{
						Internal: false,
						Name:     f,
					}
				}
				if !Config.GlobalPortList[p].Internal {
					log.Println("Creating server for:", f, "on port:", p)
					s := createServer(p)
					sd := ServerDefinition{
						Running: true,
						Server:  s,
					}
					portSet[p] = sd
				}
				portDef := Config.GlobalPortList[p]
				portDef.Internal = true
				portDef.Name = f
				Config.GlobalPortList[p] = portDef
				holdFile := lockDir + Config.Developer + "/" + f + ".hold"
				if _, err := os.Stat(holdFile); err != nil {
					log.Println("Create hold file:", holdFile)
					f, err := os.Create(holdFile)
					if err != nil {
						log.Printf("Failed to create lock file: %v", err)
					} else {
						f.Close()
					}
				}
				sd := portSet[p]
				sd.Running = true
				portSet[p] = sd
			} else {
				blockFile := lockDir + Config.Developer + "/" + f + ".blocked"

				log.Println("Server already exists for:", f, "on port:", p)
				log.Println("Create blocked file:", blockFile)
				if _, err := os.Stat(blockFile); err != nil {
					f, err := os.Create(blockFile)
					if err != nil {
						log.Printf("Failed to create lock file: %v", err)
					} else {
						f.Close()
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("Error walking the path %q: %v\n", lockDir, err)
	}
}

func removeLockedFiles(port string) {
	lockDir := "/tmp/.runtime/global_ports/" + Config.Developer + "/"
	err := filepath.Walk(lockDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.Contains(info.Name(), "."+port+".") {
			if err := os.Remove(path); err != nil {
				log.Printf("Failed to remove lock file for reserved port %s: %v", path, err)
			}

		}
		return nil
	})
	if err != nil {
		log.Printf("Error walking the path %q: %v\n", lockDir, err)
	}
}

func main() {
	// Remove the lock directory recursively before starting main loop
	lockDir := "/tmp/.runtime/global_ports/" + Config.Developer + "/"
	if err := os.RemoveAll(lockDir); err != nil {
		log.Fatalf("Failed to remove lock directory recursively: %v", err)
	}

	if err := os.MkdirAll(lockDir, 0755); err != nil {
		log.Fatalf("Failed to create lock directory: %v", err)
	}
	portSet := make(PortListeningSet)
	for {
		lockList := []string{}
		for port, config := range Config.GlobalPortList {
			status := checkPortIsOpened(port)
			if status == nil {
				if !config.Internal {
					config.Name = getPortName(port)
					Config.GlobalPortList[port] = config
					lockList = append(lockList, config.Name)
				}
			} else {
				removeLockedFiles(port)
			}
		}
		for port := range portSet {
			portSet[port] = ServerDefinition{
				Running: false,
				Server:  portSet[port].Server,
			}
		}
		findLockedFiles(lockList, portSet)
		_portSet := portSet
		for port := range _portSet {
			if !_portSet[port].Running {
				log.Printf("Have to stop internal server on: %v", port)
				_portSet[port].Server.Shutdown(context.Background())
				Config.GlobalPortList[port] = PortDefinition{Internal: false, Name: ""}
				delete(portSet, port)
			}
		}
		for _, name := range lockList {
			lockFile := lockDir + name + ".lock"
			if _, err := os.Stat(lockFile); err != nil {
				log.Println("Create lock file:", lockFile)
				f, err := os.Create(lockFile)
				if err != nil {
					log.Printf("Failed to create lock file: %v", err)
				} else {
					f.Close()
				}
			}
		}
		time.Sleep(1 * time.Second)
	}

}
