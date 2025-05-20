package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	// Directories
	originDir = getEnvString("ORIGIN_DIR", "origins")
	addonDir  = getEnvString("ADDON_DIR", "addons")
	// Filenames
	addonConfigName = getEnvString("ADDON_CONFIG_NAME", "addon.yml")
	dummyDir        = getEnvString("ADDON_DUMMY_DIR", "dummy")
	dummyOriginName = getEnvString("DUMMY_ORIGIN_NAME", "dummyOrigin")
	// Constant
	pollTimeout   time.Duration = getEnvInt64("POLL_TIMEOUT", 3)
	runningAddons               = make(map[string]*AddonType)
)

type AddonConfig struct {
	Envs   map[string]string `yaml:"envs"`
	Params []string          `yaml:"params"`
}

type AddonBase struct {
	Id        string
	IsOrigin  bool
	Path      string
	Timestamp string
	Config    AddonConfig
}

type AddonType struct {
	IsOrigin  bool
	IsRunning bool
	ExitCode  int
	Current   AddonBase
	Origin    AddonBase
}

type Addons map[string]*AddonType

func readAddonConfig(filename string, config *AddonConfig) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return err
	}
	return nil
}

func createDummyOrigin(addonName string) error {
	executableSource := getDummyOriginExecutablePath()
	configSource := getDummyOriginConfigPath()

	destinationDir := filepath.Dir(getOriginPath(addonName))

	// Create the directory if it doesn't exist
	cmd := exec.Command("mkdir", "-p", destinationDir)
	if err := cmd.Run(); err != nil {
		return err
	}

	// Copy the dummy to path
	cmd = exec.Command("cp", executableSource, getOriginPath(addonName))
	if err := cmd.Run(); err != nil {
		return err
	}
	cmd = exec.Command("cp", configSource, getOriginDir(addonName))
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func findAddons() Addons {
	var addons Addons = make(map[string]*AddonType)
	cmd := exec.Command("find", addonDir, "-type", "f", "-executable")
	output, err := cmd.Output()
	if err != nil {
		return Addons{}
	}

	executables := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, execPath := range executables {
		dirName := filepath.Base(filepath.Dir(execPath))
		execName := filepath.Base(execPath)

		if dirName != execName {
			continue
		}

		addon := AddonType{}
		currentAddonInfo, _ := os.Stat(getAddonPath(execName))
		current_conf := getAddonDir(execName) + "/" + addonConfigName
		addon.Current.Id = getAddonPath(execName)
		addon.Current.IsOrigin = false
		addon.Current.Path = getAddonPath(execName)
		addon.Current.Timestamp = currentAddonInfo.ModTime().String()

		if _, err := os.Stat(current_conf); err == nil {
			readAddonConfig(current_conf, &addon.Current.Config)
		}
		if _, err := os.Stat(getOriginPath(execName)); err != nil {
			fmt.Println(NOTICE, " New addon detected: %s", execName)
			createDummyOrigin(execName)
		}
		originAddonInfo, _ := os.Stat(getOriginPath(execName))
		origin_conf := getOriginDir(execName) + "/" + addonConfigName
		addon.Origin.Id = getAddonPath(execName)
		addon.Origin.IsOrigin = true
		addon.Origin.Path = getOriginPath(execName)
		addon.Origin.Timestamp = originAddonInfo.ModTime().String()
		if _, err := os.Stat(origin_conf); err == nil {
			readAddonConfig(origin_conf, &addon.Origin.Config)
		}
		addons[execPath] = &addon
	}
	return addons
}

func (a AddonBase) startAddon() {
	fmt.Println(NOTICE, " Starting addon: %s\n", a.Path)
	cmd := exec.Command(a.Path)
	addonCmd := runningAddons[a.Id]
	addonCmd.IsOrigin = a.IsOrigin
	addonCmd.ExitCode = -1

	if len(a.Config.Envs) != 0 {
		fmt.Println(NOTICE, " Found ENV for ", a.Path, " containing: ", a.Config.Envs)
		for name, value := range a.Config.Envs {
			cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s", name, value))
		}
	}
	if len(a.Config.Params) != 0 {
		fmt.Println(NOTICE, " Found PARAMS for ", a.Path, " containing: ", a.Config.Params)
		cmd.Args = append(cmd.Args, a.Config.Params...)
	}

	// Capture standard output and standard error
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println(ERR, "[STDERR] Can not use stdout:", a.Path, err)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Println(ERR, "[STDERR] Can not use stderr:", a.Path, err)
		return
	}

	if err := cmd.Start(); err != nil {
		fmt.Println(ERR, "[STDERR] Can not start:", a.Path, err)
		return
	}

	pid := cmd.Process.Pid

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

	addonCmd.IsRunning = true
	err = cmd.Wait()
	fmt.Println(NOTICE, " exit addon:", a.Path)
	addonCmd.IsRunning = false
	addonCmd.ExitCode = 0
	if err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			addonCmd.ExitCode = exiterr.ExitCode()
		} else {
			addonCmd.ExitCode = -1
		}
	}
}

func main() {
	for {
		for k, v := range findAddons() {
			if _, ok := runningAddons[k]; !ok {
				fmt.Println(INFO, " new addon is detected here: %v\n", k)
				runningAddons[k] = v
				go v.Current.startAddon()
			} else {
				addon := runningAddons[k]
				addon.Current.Config = v.Current.Config
				addon.Origin.Config = v.Origin.Config
				if !addon.IsRunning {
					if v.Current.Timestamp != addon.Current.Timestamp || (addon.ExitCode == 0 && addon.IsOrigin == false) {
						go v.Current.startAddon()
					} else if addon.IsOrigin == true {
						go v.Current.startAddon()
					} else {
						go v.Origin.startAddon()
					}
					addon.Current.Timestamp = v.Current.Timestamp
					addon.Origin.Timestamp = v.Origin.Timestamp
				}
			}
		}
		time.Sleep(pollTimeout * time.Second)
	}
}
