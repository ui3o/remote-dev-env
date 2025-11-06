package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.senan.xyz/flagconf"
)

var (
	Config = RuntimeConfig{}
)

func debugHeader(username string) string {
	return fmt.Sprintf("[%s] ", username)
}

func init() {
	flag.CommandLine.Init("env_param_admin_addon", flag.ExitOnError)

	flag.IntVar(&Config.Port, "port", 10113, "Port(10113)")
	flag.StringVar(&Config.TemplateRootPath, "template_root_path", "", "")
	flag.StringVar(&Config.DomainPath, "domain_path", "", "")

	if value, ok := os.LookupEnv("PORT_ADMIN"); ok {
		if portInt, err := strconv.Atoi(value); err == nil {
			Config.Port = portInt
		} else {
			log.Printf("[INIT] Invalid PORT_ADMIN value: %v\n", err)
		}
	}

	flag.Parse()
	flagconf.ParseEnv()

	confJson, err := json.MarshalIndent(Config, "", "  ")
	if err != nil {
		log.Println("[INIT] Failed to marshal config to JSON:", err)
	} else {
		log.Println("[INIT] RuntimeConfig JSON:", string(confJson))
	}
	log.Println("[INIT] TemplateRootPath", Config.TemplateRootPath)
}

func main() {
	r := gin.Default()
	r.LoadHTMLFiles(Config.TemplateRootPath+"static/admin.html",
		Config.TemplateRootPath+"static/admin.js")
	r.NoRoute(func(c *gin.Context) {
		log.Println("[REQ_START] Handle request => |", c.Request.Host, "|", c.Request.URL.Path)
		switch c.Request.URL.Path {
		case "/issh_login_data":
			if cookieName := os.Getenv("ENV_PARAM_REVERSEPROXY_COOKIE_NAME"); cookieName != "" {
				if cookie, err := c.Cookie(cookieName); err == nil {
					data := UserLoginData{
						Cookie: cookieName + "=" + cookie,
						Domain: Config.DomainPath + "/ssh",
					}
					c.JSON(http.StatusOK, data)
				}
			}
		case "/ssh":
			sshdWs(c)
		case "/", "/static/admin.html":
			log.Println("load admin.html")
			c.HTML(200, "admin.html", gin.H{
				"template_str": "This is template string!",
			})
		case "/static/admin.js":
			log.Println("load admin.js")
			c.Header("Content-Type", "application/javascript")
			c.File("static/admin.js")
		default:
			log.Println("[ERROR] Not possible to handle this request:", c.Request.URL.Path)

		}
	})
	log.Println("[BOOT] Gin start in http mode")
	r.Run(fmt.Sprintf(":%d", Config.Port))
}
