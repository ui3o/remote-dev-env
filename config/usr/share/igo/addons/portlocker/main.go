package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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
	Idle   LockFileType = iota
	Mine                // my container using this port
	Shared              // portlocker hold this port because other user is using it
)

var LOCAL_IP = "127.0.0.1"
var GLOBAL_IP = "0.0.0.0"

var LocalIPRegexp = regexp.MustCompile(`.*(127.0.0.1:)(9[0-9]{3}) \(LISTEN\)`)
var GlobalIPRegexp = regexp.MustCompile(`.*(\*:)(9[0-9]{3}) \(LISTEN\)`)

type PortDefinition struct {
	Status LockFileType
	Server *http.Server
}

type PortStatusDefinition struct {
	LocalServer  *PortDefinition
	GlobalServer *PortDefinition
}

type PortSet map[string]PortStatusDefinition
type FileList map[string]map[string]bool

func (pd PortStatusDefinition) Construct(port string) {
	psd := Config.GlobalPortList[port]
	psd.LocalServer = &PortDefinition{}
	psd.GlobalServer = &PortDefinition{}
	Config.GlobalPortList[port] = psd
	touchFile(LOCAL_IP+"."+port, Idle)
	touchFile(GLOBAL_IP+"."+port, Idle)
}

func (pd *PortDefinition) SetStatus(status LockFileType) {
	pd.Status = status
}

func (pd *PortDefinition) SetServer(server *http.Server) {
	pd.Server = server
}

func checkPortOpened() map[string]bool {
	result := make(map[string]bool)
	cmd := exec.Command("lsof", "-i", "-P")
	if output, err := cmd.Output(); err == nil {

		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if !strings.HasPrefix(line, "portlocke") {
				matches := LocalIPRegexp.FindStringSubmatch(line)
				if len(matches) == 3 {
					result[LOCAL_IP+"."+matches[2]] = true
					continue
				}
				matches = GlobalIPRegexp.FindStringSubmatch(line)
				if len(matches) == 3 {
					result[GLOBAL_IP+"."+matches[2]] = true
					continue
				}
			}
		}
	}
	return result
}

func init() {
	flag.CommandLine.Init("env_param_portlocker_addon", flag.ExitOnError)

	Config.Developer = os.Getenv("DEVELOPER")

	lockDir := "/tmp/.runtime/global_ports/" + Config.Developer + "/"
	if err := os.RemoveAll(lockDir); err != nil {
		log.Fatalf("Failed to remove lock directory recursively: %v", err)
	}
	// Create the lock directory
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		log.Fatalf("Failed to create lock directory: %v", err)
	}
	for i := 9000; i <= 9999; i++ {
		p := strconv.Itoa(i)
		Config.GlobalPortList[p].Construct(p)
	}

	flag.Parse()
	flagconf.ParseEnv()

}

func createServer(port string, server *http.Server) *http.Server {
	if server == nil {
		srv := &http.Server{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(599)
			}),
		}
		if listen, err := net.Listen("tcp", port); err != nil {
			srv.Shutdown(context.Background())
			log.Println("Server is already running: ", port)
			return nil
		} else {
			go srv.Serve(listen)
			log.Println("Creating server for: ", port)
			return srv
		}
	}
	return server
}

func collectSharedFiles() FileList {
	found := make(FileList)
	lockDir := "/tmp/.runtime/global_ports/"
	if err := filepath.Walk(lockDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && !strings.Contains(path, Config.Developer) {
			pathParts := strings.Split(path, "/")
			user := pathParts[len(pathParts)-2]
			if _, ok := found[user]; !ok {
				found[user] = make(map[string]bool)
			}
			found[user][info.Name()] = true
		}
		return nil
	}); err != nil {
		log.Printf("Error during shared file scanning, the path %q: %v\n", lockDir, err)
	}
	return found
}

func findSharedFiles(file string, files FileList, silent bool) bool {
	for user, userFiles := range files {
		if _, ok := userFiles[file]; ok {
			if !silent {
				log.Printf("%v user has file: %v", user, file)
			}
			return true
		}
	}
	return false
}

func createFile(touchFile, ext string, removeOnCreate []string) {
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
				log.Printf("Failed to remove file %s: %v", touchFile+"."+v, err)
			}
		}
	}

}

func touchFile(port string, fileType LockFileType) {
	userDir := "/tmp/.runtime/global_ports/" + Config.Developer + "/"
	touchFile := userDir + port
	switch fileType {
	case Shared:
		createFile(touchFile, "shared", []string{"idle"})
	case Mine:
		createFile(touchFile, "mine", []string{"idle"})
	default:
		createFile(touchFile, "idle", []string{"shared", "mine"})
	}
}

func checkPort(openedPorts map[string]bool, files FileList, ip, port string, portDef *PortDefinition) {
	ipFileName := ip + "." + port
	switch portDef.Status {
	case Idle:
		if _, ok := openedPorts[ipFileName]; ok {
			portDef.SetStatus(Mine)
			touchFile(ipFileName, Mine)
			break
		}
		if findSharedFiles(ipFileName+".mine", files, false) {
			portDef.SetStatus(Shared)
			touchFile(ipFileName, Shared)
			portDef.SetServer(
				createServer(ip+":"+port, portDef.Server))
		}
	case Shared:
		if !findSharedFiles(ipFileName+".mine", files, true) {
			portDef.SetStatus(Idle)
			touchFile(ipFileName, Idle)
			if portDef.Server != nil {
				portDef.Server.Shutdown(context.Background())
				portDef.SetServer(nil)
			}
		}
	case Mine:
		if _, ok := openedPorts[ipFileName]; !ok {
			portDef.SetStatus(Idle)
			touchFile(ipFileName, Idle)
		}
	}
}

func main() {
	for {
		openedPorts := checkPortOpened()
		files := collectSharedFiles()
		for port := range Config.GlobalPortList {
			checkPort(openedPorts, files, LOCAL_IP, port, Config.GlobalPortList[port].LocalServer)
			checkPort(openedPorts, files, GLOBAL_IP, port, Config.GlobalPortList[port].GlobalServer)
		}
		time.Sleep(2 * time.Second)
	}
}
