package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"regexp"

	"strings"

	"github.com/crewjam/saml/samlsp"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/ui3o/remote-dev-env/reverseproxy/saml"
	"github.com/ui3o/remote-dev-env/reverseproxy/simple"
	"go.senan.xyz/flagconf"
)

var (
	SAMLSP            *samlsp.Middleware
	CreateUserChannel = make(chan *gin.Context)
	AllRestEndpoint   = make(map[string]*AllRestEndpointDefinition)
	AllRoutesRegexp   = make(map[string]*RouteMatch)
	Config            = RuntimeConfig{
		SAML: &saml.SAMLConf,
	}
)

type UserEnv struct {
	Storage string `json:"storage"`
	jwt.RegisteredClaims
}

const DEFAULT_USERNAME = "zzz"
const DOMAIN_COOKIE_NAME = "remote-dev-domain"
const REQ_HEADER_PROXY_USER_NAME = "req-header-proxy-user-name"
const REQ_HEADER_ROUTE_ID = "req-header-route-id"

func debugHeader(username string) string {
	return fmt.Sprintf("[%s] ", username)
}

func init() {
	flag.CommandLine.Init("env_param_reverseproxy", flag.ExitOnError)

	flag.BoolVar(&Config.UseSAMLAuth, "saml", false, "Use saml auth(default is dummy) ")
	flag.BoolVar(&Config.ReplaceSubdomainToCookie, "replace_subdomain_to_cookie", false, "Use saml auth(default is dummy) ")

	flag.IntVar(&Config.Port, "port", 10111, "Port(10111)")
	flag.IntVar(&Config.CookieAge, "age", 3600, "cookie age in sec")

	flag.IntVar(&Config.UserIdleCheckInterVal, "user_idle_check_interval", 1, "default is 1 minutes")
	flag.IntVar(&Config.UserIdleKillAfterTimeout, "user_idle_kill_after_timeout", 600, "default is 600 seconds")

	flag.StringVar(&Config.TemplateRootPath, "template_root_path", "", "")
	flag.StringVar(&Config.LocalGlobalPortList, "local_global_port_list", "ADMIN;CODE;RSH;LOCAL1;LOCAL2;HIDDEN_SSHD--GRAFANA;GLOBAL1;GLOBAL2", "ADMIN;CODE;RSH;LOCAL1;LOCAL2;HIDDEN_SSHD;...--GRAFANA;PROMETHEUS;LOKI;...")
	flag.StringVar(&Config.HomeFolderPath, "home_folder_path", "", "")

	flag.StringVar(&Config.CookieName, "cookie_name", "remote-dev-env", "")

	flag.StringVar(&Config.KeyFile, "server_key", "", "")
	flag.StringVar(&Config.CertFile, "server_cert", "", "")

	flag.StringVar(&Config.SAML.IdpMetadataURL, "saml_idpmetadataurl", "", "")
	flag.StringVar(&Config.SAML.EntityID, "saml_entityid", "", "")
	flag.StringVar(&Config.SAML.CookieName, "saml_cookiename", "", "")
	flag.StringVar(&Config.SAML.RootURL, "saml_rooturl", "", "")
	flag.StringVar(&Config.SAML.CertFile, "saml_certfile", "", "")
	flag.StringVar(&Config.SAML.KeyFile, "saml_keyfile", "", "")
	flag.StringVar(&Config.SAML.Domain, "saml_domain", "", "")
	flag.StringVar(&Config.SAML.AuthnNameIDFormat, "saml_authnnameidformat", "", "")

	flag.Parse()
	flagconf.ParseEnv()

	confJson, err := json.MarshalIndent(Config, "", "  ")
	if err != nil {
		log.Println("[INIT] Failed to marshal config to JSON:", err)
	} else {
		log.Println("[INIT] RuntimeConfig JSON:", string(confJson))
	}

	if len(Config.SAML.CookieName) > 0 {
		Config.CookieName = Config.SAML.CookieName
	}

	log.Println("[INIT] TemplateRootPath", Config.TemplateRootPath)

	if Config.UseSAMLAuth {
		if saml, err := saml.InitSAML(); err == nil {
			SAMLSP = saml
		} else {
			log.Println("[INIT] Error Init SAMLSP is >", err)
		}
	}
	userCreatorInit()
	userContainerRemoverInit()
}

func main() {
	Config.LocalGlobalPortList = strings.TrimSpace(Config.LocalGlobalPortList)
	parts := strings.Split(Config.LocalGlobalPortList, "--")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) != 0 {
			ports := strings.Split(part, ";")
			for _, port := range ports {
				port = strings.TrimSpace(port)
				AllRoutesRegexp[port] = &RouteMatch{
					Regex: regexp.MustCompile(`^` + strings.ToLower(port) + `.`),
					Id:    port,
				}
			}
		}
	}

	r := gin.Default()
	r.LoadHTMLFiles(Config.TemplateRootPath + "simple/auth.html")
	r.NoRoute(func(c *gin.Context) {
		accept := c.Request.Header.Get("Accept")
		documentRequest := false
		if strings.Contains(accept, "text/html") {
			documentRequest = true
		}
		log.Println("[REQ_START] Handle request => |", c.Request.Host, "|", c.Request.URL.Path, "| isDocument=", documentRequest)

		if strings.Contains(c.Request.URL.Path, ".js") {
			c.Header("Content-Type", "application/javascript")
		}

		if user := readUser(c); !user.IsValid {
			log.Println(debugHeader(user.Name), "Handle anonymous user")
			if Config.UseSAMLAuth {
				if strings.HasPrefix(c.Request.URL.Path, "/saml/") {
					log.Println(debugHeader(user.Name), "SAMLSP handle /saml/")
					SAMLSP.ServeHTTP(c.Writer, c.Request)
				} else {
					_, err := SAMLSP.Session.GetSession(c.Request)
					log.Println(debugHeader(user.Name), "SAMLSP err >", err)
					if err == samlsp.ErrNoSession {
						log.Println(debugHeader(user.Name), "SAMLSP HandleStartAuthFlow")
						SAMLSP.HandleStartAuthFlow(c.Writer, c.Request)
					}
				}
			} else {
				if strings.HasPrefix(c.Request.URL.Path, "/saml/") {
					var req simple.JWTUser
					if err := c.ShouldBindJSON(&req); err != nil {
						log.Println(debugHeader(user.Name), "Catch ShouldBindJSON err >", err)
						c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
						return
					}
					cookie, err := simple.Encode(req)
					if err == nil {
						createCookie(c, user, Config.CookieName, cookie)
						c.JSON(200,
							gin.H{
								"status": "success",
							})
						return
					} else {
						c.JSON(500,
							gin.H{
								"status": "failed",
							})
						return
					}
				} else {
					log.Println(debugHeader(user.Name), "load auth.html")
					c.HTML(200, "auth.html", gin.H{
						"title": "Welcome to Gin",
					})
					return
				}
			}
		} else {
			modifyAccessFile(c, user.Name)
			log.Println(debugHeader(user.Name), "Handle logged in user and start to findRoute")
			if len(user.RouteId) > 0 {
				log.Println(debugHeader(user.Name), "findRoute has found a route")
				HandleRequest(user, c)
			} else {
				log.Println(debugHeader(user.Name), "No route found for logged in user!!!")
				return
			}
		}
	})

	if len(Config.CertFile) > 0 && len(Config.KeyFile) > 0 {
		log.Println("[BOOT] Gin start in https mode")
		if err := r.RunTLS(fmt.Sprintf(":%d", Config.Port), Config.CertFile, Config.KeyFile); err != nil {
			log.Fatal("[BOOT] Failed to run HTTPS server: ", err)
		}
	} else {
		log.Println("[BOOT] Gin start in http mode")
		r.Run(fmt.Sprintf(":%d", Config.Port))
	}

}
