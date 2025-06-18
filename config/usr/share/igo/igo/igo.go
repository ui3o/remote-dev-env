package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	// Directories
	igoRootPath = "/usr/share/igo"
	originDir   = filepath.Join(igoRootPath, ".runtime/origins")
	unitDir     = filepath.Join(igoRootPath, ".runtime/units")
	runDir      = filepath.Join(igoRootPath, ".runtime/run")
	addonDir    = filepath.Join(igoRootPath, "addons")
	// Filenames
	addonConfigName = getEnvString("IGO_UNIT_CONFIG", "config.py")
	// Constant
	pollTimeout   time.Duration = getEnvInt64("IGO_POLL_TIMEOUT", 3)
	runningAddons               = make(map[string]*AddonType)
	igoGrpId      int
	activeKillers sync.Map
	Config        IgoConfig
)

type IgoConfig struct {
	Debug bool
}

type RunnableConfig struct {
	Timer int           `json:"timer"`
	Start RunnableProps `json:"start"`
	Stop  RunnableProps `json:"stop"`
}

type RunnableProps struct {
	RestartCount int               `json:"restartCount"`
	Envs         map[string]string `json:"envs"`
	Params       []string          `json:"params"`
	Wd           string            `json:"wd"`
}

type AddonBase struct {
	Id         string
	IsOrigin   bool
	StartPath  string
	StopPath   string
	Timestamp  string
	ConfigPath string
	Config     RunnableConfig
}

type AddonType struct {
	IsOrigin   bool
	IsAddon    bool
	IsRunning  bool
	IsStopping bool
	ExitCode   int
	Pid        int
	Current    AddonBase
	Origin     AddonBase
	User       *user.User
	Name       string
}

func DebugPrintln(a ...any) {
	if Config.Debug {
		fmt.Println(a...)
	}
}

type Addons map[string]*AddonType

func (a *AddonBase) readRunnableConfig(path string, restartType RestartType) {
	// (done) todo add env to python why to read
	currentConfigPath := filepath.Dir(path)
	currentConfigPathCopy := filepath.Dir(path)
	var currentConfigOut string
	var runBase string
	if a.IsOrigin {
		runBase = strings.ReplaceAll(currentConfigPathCopy, "origins", "run/origins")
	} else {
		runBase = strings.ReplaceAll(currentConfigPathCopy, "units", "run")
	}
	currentConfigOut = filepath.Join(runBase, "config.json")
	// Identation is necessary to keep because of python !!!
	pythonConfigTemplate := fmt.Sprintf(`import sys;
from pathlib import Path;
sys.path.insert(0, "%s");
import config;
p=Path("%s");
p.parent.mkdir(parents=True, exist_ok=True);
import json;
p.write_text(json.dumps(config.conf))`, currentConfigPath, currentConfigOut)
	cmd := exec.Command("python3", "-c", pythonConfigTemplate)
	// Set IGO_STATE_START env variable based on restartType
	stateStart := "false"
	if restartType == Start {
		stateStart = "true"
	}
	cmd.Env = append(os.Environ(), "IGO_STATE_START="+stateStart, "PYTHONDONTWRITEBYTECODE=1")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("[IGO] err Could not run config for: ", currentConfigPath, err)
		return
	}

	// Read output from .runtime/config
	rawConfig, err := os.ReadFile(currentConfigOut)
	if err != nil {
		fmt.Println("[IGO] err Could not read config output for: ", currentConfigOut, err)
		return
	}

	err = json.Unmarshal([]byte(rawConfig), &a.Config)
	if err != nil {
		fmt.Println("[IGO] err Could not parse config for: ", currentConfigOut, " err:", err)
	}
}

func findRunnables() Addons {
	var addons Addons = make(map[string]*AddonType)
	cmd := exec.Command("find", unitDir, "-follow", "-type", "f", "-executable", "-name", "*.start")
	output, err := cmd.Output()
	DebugPrintln("find output > ", string(output))
	if err != nil {
		fmt.Println("[IGO] ERROR findRunnables() could not give find command err:", err)
		return Addons{}
	}

	executables := strings.Split(strings.TrimSpace(string(output)), "\n")
	DebugPrintln("executables > ", executables)

	if len(executables) == 0 {
		fmt.Println("[IGO] findRunnables() no addon or unit found! unitDir: ", unitDir)
	}

	for _, execPath := range executables {
		dirName := filepath.Base(filepath.Dir(execPath))
		execName := filepath.Base(execPath)

		DebugPrintln("detect execName > ", execName)
		DebugPrintln("dirName > ", dirName)
		if dirName+".start" != execName {
			continue
		}
		DebugPrintln("detect execName matched!")
		addon := AddonType{}
		addon.Name = dirName
		addonTimestampInfo, _ := os.Stat(execPath)
		addonStopPath := strings.ReplaceAll(execPath, ".start", ".stop")
		confPath := strings.ReplaceAll(execPath, execName, addonConfigName)
		addon.Current.Id = execPath
		addon.Current.IsOrigin = false
		addon.Current.StartPath = execPath
		addon.Current.Timestamp = addonTimestampInfo.ModTime().String()

		if _, err := os.Stat(confPath); err == nil {
			addon.Current.ConfigPath = confPath
		}
		if _, err := os.Stat(addonStopPath); err == nil {
			addon.Current.StopPath = addonStopPath
		}
		addon.IsAddon = strings.Contains(execPath, "addons")
		if addon.IsAddon {
			DebugPrintln("exec type is addon")
			originExecPath := strings.ReplaceAll(execPath, "units/addons", "origins")
			if _, err := os.Stat(originExecPath); err == nil {
				addonTimestampInfo, _ := os.Stat(originExecPath)
				addonStopPath := strings.ReplaceAll(originExecPath, ".start", ".stop")
				confPath = strings.ReplaceAll(originExecPath, execName, addonConfigName)
				addon.Origin.Id = execPath // the id is the same as the Current for a reason, it is used as a lookup in the runningAddons.
				addon.Origin.IsOrigin = true
				addon.Origin.StartPath = originExecPath
				addon.Origin.Timestamp = addonTimestampInfo.ModTime().String()
				if _, err := os.Stat(confPath); err == nil {
					addon.Origin.ConfigPath = confPath
				}
				if _, err := os.Stat(addonStopPath); err == nil {
					addon.Origin.StopPath = addonStopPath
				}
			}
		} else {
			DebugPrintln("exec type is unit")
			username := extractUserFromStartPath(addon.Current.StartPath)
			linuxUser, err := user.Lookup(username)
			if err == nil {
				// Check file owner matches user
				fileInfo, err := os.Lstat(addon.Current.StartPath)
				if err == nil {
					if stat, ok := fileInfo.Sys().(*syscall.Stat_t); ok {
						uid, _ := strconv.Atoi(linuxUser.Uid)
						if int(stat.Uid) != uid {
							fmt.Printf("[IGO] Security check failed: owner of %s does not match user %s (uid %d), NOT starting unit. If this is a symlink from /etc, ensure the .start file is owned by root.\n", addon.Current.StartPath, linuxUser.Username, uid)
							continue
						}
					}
				}
				addon.User = linuxUser
			} else {
				fmt.Println("[IGO] Could not get user from path: ", addon.Current.StartPath)
			}
		}
		addons[execPath] = &addon
	}
	return addons
}

func extractUserFromStartPath(path string) string {
	const prefix = ".runtime/units/"
	idx := strings.Index(path, prefix)
	if idx == -1 {
		return ""
	}
	rest := path[idx+len(prefix):]
	slashIdx := strings.Index(rest, "/")
	if slashIdx == -1 {
		return rest
	}
	return rest[:slashIdx]
}

type RestartType int

const (
	Start RestartType = iota
	Stop
)

func (a AddonBase) getCurrentRestartCount(restartType RestartType) int {
	var unitPath string
	var filePostfix string
	switch restartType {
	case Start:
		unitPath = a.StartPath
		filePostfix = ".start"
	case Stop:
		unitPath = a.StopPath
		filePostfix = ".stop"
	}

	var runPath string
	if a.IsOrigin {
		runPath = strings.ReplaceAll(unitPath, "origins", "run/origins")
	} else {
		runPath = strings.ReplaceAll(unitPath, "units", "run")
	}
	runPath = filepath.Dir(runPath)

	cmd := exec.Command("find", runPath, "-type", "f", "-name", "*"+filePostfix+".*")
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("[IGO] ", NOTICE, " did not find status file at: ", runPath, " ", err)
		return 0
	}

	lines := strings.Split(string(output), "\n")
	regexString := fmt.Sprintf("%s%s", filePostfix, `\.(\d+)$`)
	re := regexp.MustCompile(regexString)
	var numbers []int
	for _, line := range lines {
		matches := re.FindStringSubmatch(line)
		if len(matches) == 2 {
			num, err := strconv.Atoi(matches[1])
			if err == nil {
				numbers = append(numbers, num)
			}
		}
	}

	if len(numbers) == 0 {
		return 0
	}

	sort.Ints(numbers)
	return numbers[len(numbers)-1]
}

func (a AddonBase) incrementRestartCount(restartType RestartType) {
	addon := runningAddons[a.Id]
	var unitPath string
	var filePostfix string
	switch restartType {
	case Start:
		unitPath = a.StartPath
		filePostfix = ".start."
	case Stop:
		unitPath = a.StopPath
		filePostfix = ".stop."
	}

	if unitPath == "" {
		return
	}
	addonName := filepath.Base(filepath.Dir(unitPath))
	currentCount := a.getCurrentRestartCount(restartType)
	newCount := currentCount + 1

	runPath := ""
	if a.IsOrigin {
		runPath = strings.ReplaceAll(unitPath, "origins", "run/origins")
	} else {
		runPath = strings.ReplaceAll(unitPath, "units", "run")
	}
	runPath = filepath.Dir(runPath)

	newFileName := fmt.Sprintf("%s%s%d", addonName, filePostfix, newCount)
	newFilePath := filepath.Join(runPath, newFileName)

	err := touchFile(newFilePath, addon.User)
	if err != nil {
		fmt.Println("[IGO] ", ERR, "Failed to create new restart file:", err)
	}
}

func (a AddonBase) resetRestartCount() {
	var unitPath string
	if a.StartPath != "" {
		unitPath = a.StartPath
	} else if a.StopPath != "" {
		unitPath = a.StopPath
	} else {
		fmt.Println("[IGO] ", ERR, " The unit has no start or stop path, could not resetRestartCount()")
		return
	}

	runPath := ""
	if a.IsOrigin {
		runPath = strings.ReplaceAll(unitPath, "origins", "run/origins")
	} else {
		runPath = strings.ReplaceAll(unitPath, "units", "run")
	}
	runPath = filepath.Dir(runPath)

	files, err := os.ReadDir(runPath)
	if err != nil {
		return
	}
	addonName := filepath.Base(filepath.Dir(unitPath))
	startPrefix := fmt.Sprintf("%s.start.", addonName)
	stopPrefix := fmt.Sprintf("%s.stop.", addonName)
	for _, f := range files {
		name := f.Name()
		if strings.HasPrefix(name, startPrefix) || strings.HasPrefix(name, stopPrefix) {
			os.Remove(filepath.Join(runPath, name))
		}
	}
}

func (a *AddonBase) startAndRetry() {
	addonCmd := runningAddons[a.Id]
	for {
		a.startAddon()
		// no retry then return
		if a.Config.Start.RestartCount == 0 {
			break
		}

		// executed successfully no need to retry
		if runningAddons[a.Id].ExitCode == 0 {
			break
		}

		// stopping status -> no need to retry
		if runningAddons[a.Id].IsStopping {
			break
		}

		maxRetry := a.Config.Start.RestartCount
		currentRetry := a.getCurrentRestartCount(Start)
		if currentRetry > maxRetry {
			fmt.Println("[IGO] Max retry count (", maxRetry, ") exceeded")
			break
		}

		time.Sleep(1 * time.Second)
	}
	if runningAddons[a.Id].IsStopping {
		defer delete(runningAddons, a.Id)
	}
	addonCmd.IsRunning = false
}

func touchFile(path string, user *user.User) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0775); err != nil {
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
		//                                            path is like			 .runtime/run/user/unitname/unitname.start.1
		os.Chown(runUserDir, uid, gid)
		os.Chown(runUserUnitDir, uid, gid)
		os.Chown(path, uid, gid)
	}
	return file.Close()
}

// TODO: create an enum for "origin", "addon" and "unit"
func (a *AddonBase) getEnvTagForProcess(addonCmd *AddonType) map[string]string {
	envTags := make(map[string]string)
	if addonCmd.IsAddon {
		if addonCmd.IsOrigin {
			envTags["IGO_PROCESS_TYPE"] = "origin"
		} else {
			envTags["IGO_PROCESS_TYPE"] = "addon"
		}
		envTags["IGO_PROCESS_USER"] = "root"
	} else {
		if addonCmd.User == nil {
			fmt.Println("[IGO] Unit user not found, path:", a.StartPath)
			return nil
		}
		envTags["IGO_PROCESS_USER"] = addonCmd.User.Username
		envTags["IGO_PROCESS_TYPE"] = "unit"
	}
	envTags["IGO_PROCESS_NAME"] = addonCmd.Name
	return envTags
}

func mergeMaps(src map[string]string, dst map[string]string) map[string]string {
	result := dst
	for k, v := range src {
		//if the key does not already exist in dst add it.
		if _, ok := result[k]; !ok {
			result[k] = v
		}
	}
	return result
}

func (a *AddonBase) startAddon() {
	fmt.Printf("[IGO] %s Starting addon: %s, ID %s\n", NOTICE, a.StartPath, a.Id)
	addonCmd := runningAddons[a.Id]
	addonCmd.IsStopping = false
	addonCmd.IsOrigin = a.IsOrigin
	addonCmd.ExitCode = -1
	runCmd := func(restartType RestartType) *exec.Cmd {
		var execPath string
		var execConf *RunnableProps
		switch restartType {
		case Start:
			execPath = a.StartPath
			execConf = &a.Config.Start
		case Stop:
			execPath = a.StopPath
			execConf = &a.Config.Stop
		}

		if len(execPath) == 0 {
			return nil
		}
		cmd := exec.Command(execPath)

		envTags := a.getEnvTagForProcess(addonCmd)

		// read start config
		a.readRunnableConfig(execPath, restartType)
		if len(execConf.Envs) != 0 {
			//if we have envs from config we add it to envTags
			mergeMaps(execConf.Envs, envTags)
		}
		osEnv := os.Environ()
		for name, value := range envTags {
			osEnv = append(osEnv, fmt.Sprintf("%s=%s", name, value))
		}
		cmd.Env = osEnv
		if len(execConf.Params) != 0 {
			cmd.Args = append(cmd.Args, execConf.Params...)
		}

		if len(execConf.Wd) != 0 {
			cmd.Dir = execConf.Wd
		}

		// Capture standard output and standard error
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			fmt.Println("[IGO] ", ERR, "[STDERR] Can not use stdout:", execPath, err)
			return nil
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			fmt.Println("[IGO] ", ERR, "[STDERR] Can not use stderr:", execPath, err)
			return nil
		}

		// igo is set as the group of all processes started by igo
		if addonCmd.User != nil {
			uid, _ := strconv.Atoi(addonCmd.User.Uid)
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: uint32(uid),
					Gid: uint32(igoGrpId),
				},
			}
		} else {
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Gid: uint32(igoGrpId),
				},
			}
		}

		if err := cmd.Start(); err != nil {
			fmt.Println("[IGO] ", ERR, "[STDERR] Can not start:", execPath, err)
			return nil
		}

		pid := cmd.Process.Pid
		addonCmd.Pid = pid

		// capture and print the output of the executable in a separate goroutine
		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				fmt.Printf("%d [STDOUT] %s\n", pid, scanner.Text())
			}
		}()

		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				fmt.Printf("%d [STDERR] %s\n", pid, scanner.Text())
			}
		}()
		return cmd
	}
	cmd := runCmd(Start)
	if cmd != nil {
		addonCmd.IsRunning = true
		a.incrementRestartCount(Start)
		err := cmd.Wait()
		// is killfile is found IsStopping will be set. If a unit exited with 0 we have to remove the whole unit symlink, so its not started again.
		if addonCmd.IsStopping || (!addonCmd.IsAddon && err == nil) {
			defer a.removeAddon()
		}
		fmt.Println("[IGO] ", NOTICE, " exit addon:", a.StartPath)
		addonCmd.ExitCode = 0
		a.incrementRestartCount(Stop)
		cmd := runCmd(Stop)
		if cmd != nil {
			cmd.Wait()
		}
		if err != nil {
			if exiterr, ok := err.(*exec.ExitError); ok {
				addonCmd.ExitCode = exiterr.ExitCode()
			} else {
				addonCmd.ExitCode = -1
			}
		} else {
			a.resetRestartCount()
		}
	}
}

func removeKillFile(addon *AddonType) {
	var runPath string
	if addon.IsOrigin {
		runPath = strings.ReplaceAll(addon.Current.StartPath, "units/addons", "run/origins")
	} else {
		runPath = strings.ReplaceAll(addon.Current.StartPath, "units", "run")
	}
	runPath = filepath.Dir(runPath)
	unitname := filepath.Base(runPath)
	killFilePath := filepath.Join(runPath, fmt.Sprintf("%s%s", unitname, "_kill"))
	if err := os.Remove(killFilePath); err != nil {
		fmt.Println("[IGO] Error could not remove the kill file for unit: ", unitname, " err: ", err)
	}
}

func (a *AddonBase) removeAddon() {
	addon := runningAddons[a.Id]
	var symlinkPath, userDir, userRunUnitDir, userRunDir string
	defer removeKillFile(addon)

	// we only want to unlink, and cleanup if its not an addon. If its an addon it should always run, and if it fails the addon's origin should start.
	if addon.IsAddon {
		return
	}
	if addon.User != nil {
		// Unit: symlink is in .runtime/units/{username}/{name}
		symlinkPath = filepath.Join(unitDir, addon.User.Username, addon.Name)
		userDir = filepath.Join(unitDir, addon.User.Username)
		userRunUnitDir = filepath.Join(runDir, addon.User.Username, addon.Name)
		userRunDir = filepath.Join(runDir, addon.User.Username)
	} else {
		return
	}

	// Remove symlink
	if err := os.Remove(symlinkPath); err == nil {
		fmt.Printf("[IGO] Symlink removed for unit %s\n", addon.Name)
		// Remove user dir if empty
		entries, err := os.ReadDir(userDir)
		if err == nil && len(entries) == 0 {
			os.Remove(userDir)
		}
	} else if !os.IsNotExist(err) {
		fmt.Printf("[IGO] Failed to remove symlink for unit %s: %v\n", addon.Name, err)
	}

	// Remove .runtime/run/username/unit directory
	if err := os.RemoveAll(userRunUnitDir); err == nil {
		fmt.Printf("[IGO] Removed run directory: %s\n", userRunUnitDir)
	}

	// Remove .runtime/run/username if empty
	runEntries, err := os.ReadDir(userRunDir)
	if err == nil && len(runEntries) == 0 {
		os.Remove(userRunDir)
	}
}

// sendSIGKILLAfterTimeout waits 30 seconds and sends SIGKILL if the process is still running
func sendSIGKILLAfterTimeout(pid int) {
	if _, exists := activeKillers.LoadOrStore(pid, true); exists {
		return // Already scheduled
	}

	go func() {
		defer activeKillers.Delete(pid)
		time.Sleep(5 * time.Second)
		_, err := os.FindProcess(pid)
		if err != nil {
			return
		}
		time.Sleep(10 * time.Second)
		_, err = os.FindProcess(pid)
		if err != nil {
			return
		}
		time.Sleep(15 * time.Second)
		proc, err := os.FindProcess(pid)
		if err != nil {
			return
		}

		// Check if the process is still alive
		if err := proc.Signal(syscall.Signal(0)); err == nil {
			fmt.Println("[IGO] Process still alive after 30s, sending SIGKILL:", pid)
			if err := proc.Signal(syscall.SIGKILL); err != nil {
				fmt.Println("[IGO] Failed to send SIGKILL to process:", pid)
			} else {
				fmt.Println("[IGO] SIGKILL sent to process:", pid)
			}
		} else {
			fmt.Println("[IGO] Process already exited before SIGKILL:", pid)
		}
	}()
}

func (a *AddonBase) watchDummy() {
	addon := runningAddons[a.Id]
	addon.IsRunning = true
	dummyPath := fmt.Sprintf("%s%s", strings.ReplaceAll(a.StartPath, "units", "run"), ".origin.dummy")
	touchFile(dummyPath, addon.User)
	fmt.Printf("[IGO] Origin was empty, remove %s to start again the addon", dummyPath)
	for {
		if _, err := os.Stat(dummyPath); err != nil {
			fmt.Println("[IGO] Dummy removed, initiating addon restart ...")
			break
		}
		time.Sleep(1 * time.Second)
	}
	addon.Current.resetRestartCount()
	addon.IsRunning = false
	addon.ExitCode = 0 // so it will try to run the addon again and not fallback to the origin that does not exist.
}

func copyAddonToOrigin() {
	cmd := exec.Command("cp", "-rf", addonDir+"/.", originDir)
	if err := cmd.Run(); err != nil {
		fmt.Println("[IGO] Could not copy addons :", addonDir, "to origins", originDir, " ", err)
		os.Exit(1)
	}
}

func symlinkAddonToRuntimeUnits() {
	symlinkPath := filepath.Join(unitDir, "addons")
	targetPath := addonDir

	// Check if symlink exists and points to the correct target
	info, err := os.Lstat(symlinkPath)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			dest, err := os.Readlink(symlinkPath)
			if err == nil && dest == targetPath {
				return // Symlink exists and is correct
			}
		}
	}
	// Create the symlink
	err = os.Symlink(targetPath, symlinkPath)
	if err != nil {
		fmt.Printf("[IGO] Could not create symlink %s -> %s: %v\n", symlinkPath, targetPath, err)
	}
}

func emptyOrigin() {
	err := os.RemoveAll(originDir)
	if err != nil {
		fmt.Println("[IGO] Could not empty origin :", originDir, " err:", err)
		os.Exit(1)
	}
}

func cleanRunFiles() {
	err := os.RemoveAll(runDir)
	if err != nil {
		fmt.Println("[IGO] Could not empty rundir :", runDir, " err:", err)
		os.Exit(1)
	}
}

func setIgoGrpId() {
	igoGrp, err := user.LookupGroup("igo")
	if err != nil {
		fmt.Println("[IGO] Could not find igo group! err:", err)
		panic(1)
	}
	gid, err := strconv.Atoi(igoGrp.Gid)
	if err != nil {
		fmt.Println("[IGO] Could not parse igo group id: ", igoGrp.Gid, " err:", err)
		panic(1)
	}
	igoGrpId = gid
}

func init() {
	flag.BoolVar(&Config.Debug, "v", false, "verbose")
	flag.Parse()
	DebugPrintln("debug mode enabled!")
	fmt.Println("[IGO] Starting ...")
	zombieInit()
	emptyOrigin()
	copyAddonToOrigin()
	symlinkAddonToRuntimeUnits()
	cleanRunFiles()
	setIgoGrpId()
}

func main() {
	for {
		DebugPrintln("find runnable cycle run..")
		for k, v := range findRunnables() {
			if _, ok := runningAddons[k]; !ok {
				fmt.Printf("[IGO] %s new addon is detected here: %v\n", INFO, k)
				runningAddons[k] = v
				go v.Current.startAndRetry()
			} else {
				addon := runningAddons[k]
				addon.Current.Config = v.Current.Config
				addon.Origin.Config = v.Origin.Config
				if !addon.IsRunning {
					// (done) todo how many retry
					// (done) todo is addon and has origin?
					if v.Current.Timestamp != addon.Current.Timestamp || (addon.ExitCode == 0 && addon.IsOrigin) {
						go v.Current.startAndRetry()
					} else if addon.IsOrigin {
						go v.Current.startAndRetry()
					} else {
						// (done) todo touch origin file
						if reflect.DeepEqual(v.Origin, AddonBase{}) {
							go v.Current.watchDummy()
						} else {
							fmt.Println("[IGO] Fallback to Origin ", v.Origin.Id)
							v.Origin.resetRestartCount()
							v.Current.resetRestartCount()
							go v.Origin.startAndRetry()
						}
					}
					addon.Current.Timestamp = v.Current.Timestamp
					addon.Origin.Timestamp = v.Origin.Timestamp
				} else {
					var runPath string
					if addon.IsOrigin {
						runPath = strings.ReplaceAll(v.Current.StartPath, "units/addons", "run/origins")
					} else {
						runPath = strings.ReplaceAll(v.Current.StartPath, "units", "run")
					}
					runPath = filepath.Dir(runPath)
					unitname := filepath.Base(runPath)
					killFilePath := filepath.Join(runPath, fmt.Sprintf("%s%s", unitname, ".kill"))
					if _, err := os.Stat(killFilePath); err != nil {
						continue
					}
					addon.IsStopping = true
					// edge case, if its a unit, and we are killing the dummy origin, then we dont want to remove the whole unit, just kill the dummy, and restart the addon.
					if addon.IsAddon && strings.Contains(killFilePath, "origins") {
						addon.IsStopping = false
					}
					procToTerm, err := os.FindProcess(addon.Pid)
					if err != nil {
						fmt.Println("[IGO] Failed to find process: ", addon.Pid, " this can happen if the process was forcefully terminated (kill)")
					}
					err = procToTerm.Signal(syscall.SIGTERM)
					if err != nil {
						fmt.Println("[IGO] Failed to terminate process: ", addon.Pid, " this can happen if the process was forcefully terminated (kill)")
					}
					sendSIGKILLAfterTimeout(addon.Pid)
				}
			}
		}
		time.Sleep(pollTimeout * time.Second)
	}
}
