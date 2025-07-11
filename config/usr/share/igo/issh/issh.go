package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.senan.xyz/flagconf"
)

var (
	Config = RuntimeConfig{}
)

type RuntimeConfig struct {
	DebugLevel int
}

type UserLoginData struct {
	Cookie string `json:"cookie"`
	Domain string `json:"domain"`
}

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	flag.CommandLine.Init("env_param_issh", flag.ExitOnError)
	flag.IntVar(&Config.DebugLevel, "debug_level", 7, "default is disabled")

	flag.Parse()
	flagconf.ParseEnv()

	zerolog.SetGlobalLevel(zerolog.Level(int32(Config.DebugLevel)))

	if confJson, err := json.MarshalIndent(Config, "", "  "); err != nil {
		log.Error().AnErr("[INIT] Failed to marshal config to JSON:", err)
	} else {
		log.Debug().Str("[INIT] RuntimeConfig JSON:", string(confJson))
	}

}
func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal().AnErr("Unable to determine home directory:", err)
	}
	downloadsDir := filepath.Join(homeDir, "Downloads")
	// On Windows, "Downloads" may be localized. Workaround: try to use the "Downloads" known folder GUID if on Windows.
	if homeDir != "" && os.PathSeparator == '\\' {
		// Attempt to resolve the Downloads folder from environment vars if available
		if downloadsEnv := os.Getenv("USERPROFILE"); downloadsEnv != "" {
			possiblePath := filepath.Join(downloadsEnv, "Downloads")
			if _, err := os.Stat(possiblePath); err == nil {
				downloadsDir = possiblePath
			}
		}
	}
	if _, err := os.Stat(downloadsDir); os.IsNotExist(err) {
		log.Fatal().Str("Downloads directory not found at:", downloadsDir)
	}
	dataPath := filepath.Join(downloadsDir, "issh_login_data.json")
	files, err := os.ReadDir(downloadsDir)
	if err != nil {
		log.Fatal().AnErr("Unable to read Downloads directory:", err)
	}
	var latestModTime int64
	var latestFile string
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		if matched, _ := filepath.Match("issh_login_data*.json", name); matched {
			info, err := file.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Unix() > latestModTime {
				latestModTime = info.ModTime().Unix()
				latestFile = name
			}
		}
	}
	if latestFile != "" {
		dataPath = filepath.Join(downloadsDir, latestFile)
	}
	data, err := os.ReadFile(dataPath)
	if err != nil {
		log.Fatal().AnErr("Unable to read data.json:", err)
	}
	log.Info().Msgf("Read %d bytes from %s", len(data), dataPath)

	var loginData UserLoginData
	if err := json.Unmarshal(data, &loginData); err != nil {
		log.Fatal().AnErr("Unable to parse data.json:", err)
	} else {
		header := make(map[string][]string)
		header["Cookie"] = []string{loginData.Cookie}
		ws, _, err := websocket.DefaultDialer.Dial(loginData.Domain, header)
		if err != nil {
			log.Fatal().AnErr("Dial error:", err)
		}
		defer ws.Close()
		// Pipe STDIN to websocket
		go func() {
			buf := make([]byte, 1024)
			for {
				n, err := os.Stdin.Read(buf)
				if err != nil {
					break
				}
				if err := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
					break
				}
			}
		}()

		// Pipe websocket to STDOUT
		for {
			_, message, err := ws.ReadMessage()
			if err != nil {
				break
			}
			os.Stdout.Write(message)
		}
	}

}
