package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	linuxUser           *user.User
	runAll              bool
	filter              string
	unitConfigToLookFor = getEnvString("IGO_UNIT_CONFIG", "config.py")
	pollTimeout         = getEnvString("IGO_POLL_TIMEOUT", "3")
	killPatience        = getEnvString("IGO_KILL_PATIENCE", "3")
	igoRootPath         = getEnvString("IGO_ROOT_PATH", "/home/podman/ss/pilot_zoli/go/goapp/igo") // when igo starts it sets IGO_ROOT_PATH
	igoUnitSymlinkPath  = path.Join(igoRootPath, ".runtime/units")
	igoRunPath          = path.Join(igoRootPath, ".runtime/run")
)

func getEnvString(env string, def string) string {
	if val, ok := os.LookupEnv(env); ok {
		return val
	}
	return def
}

type ProcessInfo struct {
	PID  int
	User string
	Type string
	Name string
	Cmd  string
}

func findProcessDummies() []ProcessInfo {
	cmd := exec.Command("find", igoRunPath, "-type", "f", "-name", "*.origin.dummy")
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("No dumy found")
		return nil
	}

	processDummies := []ProcessInfo{}
	foundDummies := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, foundUnit := range foundDummies {
		// Strip igoRunPath prefix
		relPath := strings.TrimPrefix(foundUnit, igoRunPath)
		relPath = strings.TrimPrefix(relPath, "/")
		parts := strings.Split(relPath, string(os.PathSeparator))
		if len(parts) < 3 {
			continue // not enough parts to parse
		}
		var unitType, unitUser, unitName string
		if parts[0] == "addons" {
			unitType = "addon"
			unitUser = "root"
			unitName = parts[1]
		} else {
			unitType = "unit"
			unitUser = parts[0]
			unitName = parts[1]
		}
		info := ProcessInfo{User: unitUser, Type: unitType, Name: unitName, Cmd: foundUnit}
		processDummies = append(processDummies, info)
	}

	return processDummies
}

// This function returns PIDs that /proc/[pid]/environ contains the given envLine in parameter.
// the envLine is the key value pair seperated with "=" in one string eg.: IGO_PROCESS_USER=zoli
func findPIDsByEnvLines(envLines []string) []ProcessInfo {
	var foundProcesses []ProcessInfo
	if len(envLines) == 0 {
		return foundProcesses
	}

	envLineSet := make(map[string]struct{}, len(envLines))
	for _, line := range envLines {
		envLineSet[line] = struct{}{}
	}

	procDir := "/proc"
	entries, err := os.ReadDir(procDir)
	if err != nil {
		fmt.Println("Failed to read /proc dir: ", err)
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pidStr := entry.Name()
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			// not a numeric directory name, so not a process id -> ignore
			continue
		}

		environPath := filepath.Join(procDir, pidStr, "environ")
		environData, err := os.ReadFile(environPath)
		if err != nil {
			// The user cannot read this environ file, does not have permission -> ignore
			continue
		}

		var found bool
		var userVal, typeVal, nameVal string
		// Environment variables are null-seperated
		envs := bytes.Split(environData, []byte{0})
		for _, envBytes := range envs {
			if len(envBytes) == 0 {
				continue // Skip empty string
			}
			envStr := string(envBytes)
			if strings.HasPrefix(envStr, "IGO_PROCESS_USER=") {
				userVal = strings.TrimPrefix(envStr, "IGO_PROCESS_USER=")
			}
			if strings.HasPrefix(envStr, "IGO_PROCESS_TYPE=") {
				typeVal = strings.TrimPrefix(envStr, "IGO_PROCESS_TYPE=")
			}
			if strings.HasPrefix(envStr, "IGO_PROCESS_NAME=") {
				nameVal = strings.TrimPrefix(envStr, "IGO_PROCESS_NAME=")
			}
			if _, ok := envLineSet[envStr]; ok {
				found = true
			}
		}
		if found {
			info := ProcessInfo{PID: pid, User: userVal, Type: typeVal, Name: nameVal}
			cmdLinePath := filepath.Join(procDir, pidStr, "cmdline")
			cmdLineData, cmdErr := os.ReadFile(cmdLinePath)
			if cmdErr == nil {
				info.Cmd = strings.TrimSpace(string(bytes.ReplaceAll(cmdLineData, []byte{0}, []byte{' '})))
			}
			foundProcesses = append(foundProcesses, info)
		}
	}

	return foundProcesses
}

/*
units []string -> array of unit names we have to start. If the -a | -all flag is set ignore units. If its empty print an error "No units specified, to run all for the user use the -a=T | -all=T flag"
Starts the units of the given user using igo.
If the -a | -all flag was used start all units of the given user
To start a unit with igo you have to symlink the unit's directory to igo's ./runtime/units/ directory.
So we have to check if its already symlinked. If it is print "Unit symlink already exists!"
If the symlink is not yet present, then create it for each unit
*/
func start(units []string) {
	if !runAll && len(units) == 0 {
		fmt.Println("No units specified, to run all for the user use the -a=T or -all=T flag")
		return
	}

	userHome := linuxUser.HomeDir
	// find units in user's home
	cmd := exec.Command("find", userHome, "-mindepth", "2", "-maxdepth", "2", "-type", "f", "-name", unitConfigToLookFor)
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("No units found for user: ", linuxUser.Name, ", make sure the unit is in the root of your home directory: ", userHome)
		return
	}

	usersUnits := make(map[string]string)
	foundUnits := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, foundUnit := range foundUnits {
		unitName := filepath.Base(filepath.Dir(foundUnit))
		usersUnits[unitName] = filepath.Dir(foundUnit)
	}

	var unitsToStart []string
	if runAll {
		for unit := range usersUnits {
			unitsToStart = append(unitsToStart, unit)
		}
	} else {
		unitsToStart = units
	}

	for _, unit := range unitsToStart {
		userUnitPath, ok := usersUnits[unit]
		if !ok {
			fmt.Printf("Unit %s NOT found for user %s\n", unit, linuxUser.Username)
			continue
		}
		symlinkPath := filepath.Join(igoUnitSymlinkPath, linuxUser.Username, unit)
		// Check if symlink already exists
		fi, err := os.Lstat(symlinkPath)
		if err == nil && fi.Mode()&os.ModeSymlink != 0 {
			fmt.Printf("Unit symlink already exists for %s!\n", unit)
			continue
		}
		// Remove any existing file (not symlink) at symlinkPath
		if err == nil {
			os.Remove(symlinkPath)
		}
		// Create symlink's parent directory
		if err := os.MkdirAll(filepath.Dir(symlinkPath), 0755); err != nil {
			fmt.Printf("Failed to create parent directory for symlink: %v\n", err)
			continue
		}
		// Create symlink
		err = os.Symlink(userUnitPath, symlinkPath)
		if err != nil {
			fmt.Printf("Failed to create symlink for unit %s: %v\n", unit, err)
			// remove the symlink's parent directory if the symlink creation fails.
			os.Remove(filepath.Dir(symlinkPath))
			continue
		}
		fmt.Printf("Symlink created for unit %s -> %s\n", unit, symlinkPath)
	}
}

type RestartType int

const (
	Start RestartType = iota
	Stop
)

func touchFile(path string, user *user.User) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0777); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	// If its a unit we have to set the owner
	if user != nil {
		uid, _ := strconv.Atoi(user.Uid)
		gid, _ := strconv.Atoi(user.Gid)
		runUserUnitDir := filepath.Dir(path)       // runUserUnitDir is like .runtime/run/user/unitname
		runUserDir := filepath.Dir(runUserUnitDir) // runUserDir is like 	 .runtime/run/user
		//                                            path is like			 .runtime/run/user/unitname/unitname_start.1
		os.Chown(runUserDir, uid, gid)
		os.Chown(runUserUnitDir, uid, gid)
		os.Chown(path, uid, gid)
	}
	return file.Close()
}

func writeStopFile(procRunDir string) error {
	unitName := filepath.Base(procRunDir) // the name of the unit is the same as the directory its in.
	stopFileName := fmt.Sprintf("%s%s", unitName, "_kill")
	newFilePath := filepath.Join(procRunDir, stopFileName)

	err := touchFile(newFilePath, linuxUser)
	if err != nil {
		fmt.Println("[ICTL] Failed to create new restart file:", err)
	}

	return nil
}

func getRunDirForProcess(processInfo ProcessInfo) string {
	if processInfo.User == "" || processInfo.Name == "" {
		fmt.Println("Insufficient process information to get process run path")
	}

	if processInfo.User == "root" {
		if processInfo.Type == "addon" {
			return path.Join(igoRunPath, "addons", processInfo.Name)
		}
		return path.Join(igoRunPath, "origins", processInfo.Name)
	}
	return path.Join(igoRunPath, processInfo.User, processInfo.Name)
}

// kill process by writing a unitname_stop.x file
func stop(procNames []string) {
	if !runAll && len(procNames) == 0 {
		fmt.Println("No units specified, to stop all for the user use the -a=T or -all=T flag")
		return
	}

	var envsToLookFor []string
	if len(procNames) > 0 {
		for _, procName := range procNames {
			envsToLookFor = append(envsToLookFor, fmt.Sprintf("IGO_PROCESS_NAME=%s", procName))
		}
	} else {
		envsToLookFor = []string{fmt.Sprintf("IGO_PROCESS_USER=%s", linuxUser.Username)}
	}

	processes := findPIDsByEnvLines(envsToLookFor)
	for _, process := range processes {
		procRunDir := getRunDirForProcess(process)
		if err := writeStopFile(procRunDir); err != nil {
			println(err)
		}
	}

	dummies := findProcessDummies()
	for _, dummy := range dummies {
		shouldStop := false
		if len(procNames) > 0 {
			for _, name := range procNames {
				if dummy.Name == name {
					shouldStop = true
					break
				}
			}
		} else {
			// If no names specified, stop all for user
			if linuxUser.Username == "root" {
				shouldStop = true
			} else if dummy.User == linuxUser.Username {
				shouldStop = true
			}
		}
		if shouldStop {
			dummyPath := dummy.Cmd
			if err := os.Remove(dummyPath); err != nil {
				fmt.Printf("Failed to remove dummy file %s: %v\n", dummyPath, err)
			} else {
				fmt.Printf("Removed dummy file for unit %s (%s)\n", dummy.Name, dummyPath)
			}
		}
	}

	if len(processes) == 0 {
		return
	}

	timeout, _ := strconv.Atoi(pollTimeout)
	maxWait, _ := strconv.Atoi(killPatience) // seconds

	// dont hold up the prompt
	// start a goroutine to check and kill after patience
	go func(processes []ProcessInfo, timeout, maxWait int) {
		waited := 0
		stillRunning := make(map[int]ProcessInfo)
		time.Sleep(time.Duration(timeout) * time.Second)
		for _, process := range processes {
			stillRunning[process.PID] = process
		}
		for waited < maxWait && len(stillRunning) > 0 {
			for pid := range stillRunning {
				cmd := exec.Command("ps", "-p", fmt.Sprintf("%d", pid))
				out, err := cmd.Output()
				if err != nil || !strings.Contains(string(out), fmt.Sprintf("%d", pid)) {
					// process exited
					delete(stillRunning, pid)
				}
			}
			if len(stillRunning) == 0 {
				break
			}
			time.Sleep(time.Duration(timeout) * time.Second)
			waited += timeout
		}
		for pid := range stillRunning {
			fmt.Printf("Process %d did not stop after %d seconds, sending SIGKILL\n", pid, maxWait)
			syscall.Kill(pid, syscall.SIGKILL)
		}
	}(processes, timeout, maxWait)
}

func restart(procNames []string) {
	fmt.Println("NOT implemented!!")
	return
	//stop(procNames)
	//time.Sleep(1 * time.Second)
	//start(procNames)
}

func getProcessStartTime(pid int) string {
	cmd := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "start=")
	out, err := cmd.Output()
	if err != nil {
		return "-"
	}
	return strings.TrimSpace(string(out))
}

func listUnits() {
	var envsToLookFor []string
	if linuxUser.Username == "root" {
		envsToLookFor = []string{"IGO_PROCESS_TYPE=origin", "IGO_PROCESS_TYPE=addon", "IGO_PROCESS_TYPE=unit"}
	} else {
		envsToLookFor = []string{fmt.Sprintf("IGO_PROCESS_USER=%s", linuxUser.Username)}
	}

	processes := findPIDsByEnvLines(envsToLookFor)
	dummies := findProcessDummies()

	if len(processes) == 0 && len(dummies) == 0 {
		fmt.Println("No running units found.")
		return
	}

	// Pretty print
	fmt.Printf("%-12s %-8s %-12s %-8s %-12s %s\n", "Started", "PID", "User", "Type", "Name", "Cmd")
	fmt.Println(strings.Repeat("-", 80))
	for _, proc := range processes {
		user := proc.User
		if user == "" {
			user = "-" //linuxUser.Username
		}
		typ := proc.Type
		if typ == "" {
			typ = "-"
		}
		name := proc.Name
		if name == "" {
			name = "-"
		}
		cmd := proc.Cmd
		if len(cmd) > 40 {
			cmd = cmd[:37] + "..."
		}
		started := getProcessStartTime(proc.PID)
		fmt.Printf("%-12s %-8d %-12s %-8s %-12s %s\n", started, proc.PID, user, typ, name, cmd)
	}
	// print any found dummies as well.
	for _, dummy := range dummies {
		if linuxUser.Username != "root" && linuxUser.Username != dummy.User {
			continue
		}
		user := dummy.User
		if user == "" {
			user = "-"
		}
		typ := dummy.Type
		if typ == "" {
			typ = "-"
		}
		name := dummy.Name
		if name == "" {
			name = "-"
		}
		// Mark as dummy, no PID or Cmd
		fmt.Printf("%-12s %-8s %-12s %-8s %-12s %s\n", "[DUMMY]", "-", user, typ, name, "[DUMMY]")
	}

}

func init() {
	executingUser, err := user.Current()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	username := ""
	flag.StringVar(&username, "user", executingUser.Username, "The user who's units should be started")
	flag.StringVar(&username, "u", executingUser.Username, "The user who's units should be started (shorthand)")
	flag.BoolVar(&runAll, "all", false, "True if we want to run all units of the user (can be: 1, 0, t, f, T, F, true, false, TRUE, FALSE, True, False)")
	flag.BoolVar(&runAll, "a", false, "True if we want to run all units of the user (shorthand) (can be: 1, 0, t, f, T, F, true, false, TRUE, FALSE, True, False)")
	flag.StringVar(&filter, "filter", "", "Filter for type of unit, possible values: 'origin', 'addon', 'unit'")
	flag.StringVar(&filter, "f", "", "Filter for type of unit (shorthand), possible values: 'origin', 'addon', 'unit'")
	// TODO: implement filter by type. Will be usseful for root user only.

	flag.Parse()

	linuxUser, err = user.Lookup(username)
	if err != nil {
		fmt.Println("Invalid user received in flag, fallback to executor: ", executingUser.Username, " ", err)
		linuxUser = executingUser
	}

	// Drop privileges to the target user.
	gid, _ := strconv.Atoi(linuxUser.Gid)
	if err := syscall.Setgid(gid); err != nil {
		fmt.Println("Failed to set GID to user:", linuxUser.Username, " err:", err)
		os.Exit(1)
	}
	uid, _ := strconv.Atoi(linuxUser.Uid)
	if err := syscall.Setuid(uid); err != nil {
		fmt.Println("Failed to drop privileges to user:", linuxUser.Username, " err:", err)
		os.Exit(1)
	}
}

func help() {
	fmt.Println("Usage: ictl -u=user -a=T/F [start|stop|restart|list|run]")
}

func main() {
	args := flag.Args()
	if len(args) < 1 {
		help()
		os.Exit(1)
	}
	switch action := args[0]; action {
	case "start":
		start(args[1:])
	case "stop":
		stop(args[1:])
	case "restart":
		restart(args[1:])
	case "list":
		listUnits()
	case "status":
		listUnits()
	default:
		help()
		os.Exit(1)
	}
}
