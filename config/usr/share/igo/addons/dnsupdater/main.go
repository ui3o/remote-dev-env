package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	"go.senan.xyz/flagconf"
)

type RuntimeConfig struct {
	DebugLevel          int
	ResolvConfPath      string
	CustomDNSServerName string
}

var (
	Config = RuntimeConfig{}
)

var (
	TermColorReset  = "\033[0m"
	TermColorRed    = "\033[31m"
	TermColorGreen  = "\033[32m"
	TermColorBlue   = "\033[34m"
	TermColorYellow = "\033[33m"
)

func red(s string) string {
	return TermColorRed + s + TermColorReset
}

func green(s string) string {
	return TermColorGreen + s + TermColorReset
}

func logger(title string, v ...any) {
	var loggerMu sync.Mutex
	loggerMu.Lock()
	defer loggerMu.Unlock()
	fmt.Printf("[%s] %s | ",
		title,
		time.Now().Format("2006/01/02 - 15:04:05"),
	)
	for _, val := range v {
		fmt.Print(val)
	}
	fmt.Print("\n")
}

func findOriginalDNS(resolvConfPath string) (nameserverIP string) {
	file, err := os.Open(resolvConfPath)
	if err != nil {
		logger(red("ERR"), "Failed to open "+resolvConfPath+" :", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		splitLine := strings.Split(line, " ")
		if splitLine[0] == "nameserver" {
			nameserverIP = strings.TrimSpace(splitLine[1])
			break
		}
	}
	if err := scanner.Err(); err != nil {
		logger(red("ERR"), "Error reading "+resolvConfPath+" :", err)
		return
	}
	if Config.DebugLevel > 0 {
		if nameserverIP != "" {
			logger("RESOLV", "Found nameserver IP:", nameserverIP)
		} else {
			logger("RESOLV", "No nameserver IP found in "+resolvConfPath)
		}
	}
	return nameserverIP
}

func init() {
	flag.CommandLine.Init("env_param_dnsupdater", flag.ExitOnError)

	flag.IntVar(&Config.DebugLevel, "debug", 0, "Enable debug mode")

	flag.StringVar(&Config.CustomDNSServerName, "custom_dns_server_name", "dns-gateway", "")
	flag.StringVar(&Config.ResolvConfPath, "resolv_conf_path", "/etc/resolv.conf", "Path to resolv.conf")

	flag.Parse()
	flagconf.ParseEnv()

	if confJson, err := json.MarshalIndent(Config, "", "  "); err != nil {
		logger(red("ERR"), "Failed to marshal config to JSON:", err)

	} else {
		logger("INI", "RuntimeConfig JSON:", string(confJson))
	}

	if _, err := os.Stat(Config.ResolvConfPath + ".old"); err == nil {
		logger("WAR", Config.ResolvConfPath+".old exists")
	} else {
		input, err := os.ReadFile(Config.ResolvConfPath)
		if err != nil {
			logger(red("ERR"), "Failed to read "+Config.ResolvConfPath+":", err)
		} else {
			err = os.WriteFile(Config.ResolvConfPath+".old", input, 0644)
			if err != nil {
				logger(red("ERR"), "Failed to write "+Config.ResolvConfPath+".old:", err)
			} else {
				logger("INI", Config.ResolvConfPath+" copied to "+Config.ResolvConfPath+".old")
			}
		}
	}

}

func main() {
	dnsIP := findOriginalDNS(Config.ResolvConfPath + ".old")
	for {
		c := new(dns.Client)
		r := new(dns.Msg)
		r.SetQuestion(dns.Fqdn(Config.CustomDNSServerName), dns.TypeA)

		resp, _, err := c.Exchange(r, dnsIP+":53")
		if err != nil {
			logger(red("ERR"), "Failed to forward DNS request:", err)
		} else {
			for _, ans := range resp.Answer {
				if aRecord, ok := ans.(*dns.A); ok {
					dnsGatewayIP := findOriginalDNS(Config.ResolvConfPath)
					if dnsGatewayIP != aRecord.A.String() {
						err = os.WriteFile(Config.ResolvConfPath, []byte("nameserver "+aRecord.A.String()+"\n"), 0644)
						if err != nil {
							logger(red("ERR"), "Failed to update "+Config.ResolvConfPath+":", err)
						} else {
							logger(green("DNS"), Config.ResolvConfPath+" updated with: nameserver "+aRecord.A.String())
						}
					}
					if Config.DebugLevel > 0 {
						logger(green("DNS"), "Received A record for ", Config.CustomDNSServerName, ": ", aRecord.A.String())
					}
				}
			}
		}
		time.Sleep(time.Duration(5) * time.Second)
	}
}
