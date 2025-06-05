package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"regexp"

	"net/url"
	"strings"

	"github.com/crewjam/saml/samlsp"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/ui3o/remote-dev-env/igo-reverseproxy/saml"
	"github.com/ui3o/remote-dev-env/igo-reverseproxy/simple"
)

type key int

const (
	availableRemoteId key = iota
	preHandlerCalled
)

var (
	SAMLSP          *samlsp.Middleware
	AllRestEndpoint = make(map[string]*RestEndpointDefinition)
	AllRoutesRegexp = []*RouteMatch{}
	Config          = RuntimeConfig{
		SAML: &saml.SAMLConf,
	}
)

type RuntimeConfig struct {
	CookieName             string
	Port                   int
	CertFile               string
	KeyFile                string
	SimpleAuthTemplatePath string
	UseSAMLAuth            bool
	SAML                   *saml.SAMLConfig
}

type RouteMatch struct {
	Regex *regexp.Regexp
	Id    string
}

type remote struct {
	remote  *url.URL
	reverse *httputil.ReverseProxy
}

type RestEndpointDefinition struct {
	Id         string
	Proxies    []string
	Remotes    map[string]remote
	PreHandler func(ep *RestEndpointDefinition, w http.ResponseWriter, r *http.Request)
}

type availableRemote struct {
	current string
	all     map[string]bool
}

func (p *RestEndpointDefinition) serveNextProxy(currentState bool, w http.ResponseWriter, r *http.Request) {
	ar := r.Context().Value(availableRemoteId).(*availableRemote)
	pHandlerCall := r.Context().Value(preHandlerCalled).(bool)
	ar.all[ar.current] = currentState
	for k, v := range ar.all {
		if v {
			ar.current = k

			r.URL.Host = p.Remotes[k].remote.Host
			r.Host = p.Remotes[k].remote.Host
			r.Header.Set("X-Forwarded-Host", r.Host)
			r.Header.Set("X-Forwarded-For", r.RemoteAddr)

			if !pHandlerCall {
				log.Println("PreHandler for ", p.Id)
				p.PreHandler(p, w, r)
			}

			ctx := context.WithValue(r.Context(), preHandlerCalled, true)
			if strings.ToLower(r.Header.Get("Connection")) == "upgrade" &&
				strings.ToLower(r.Header.Get("Upgrade")) == "websocket" {
				// Handle WebSocket upgrade
				var err error
				reqHeader := http.Header{
					"host": []string{p.Remotes[k].remote.Host},
				}
				subProtocols := r.Header.Get("sec-websocket-protocol")
				if len(subProtocols) > 0 {
					reqHeader["sec-websocket-protocol"] = []string{subProtocols}
				}
				upgrader := websocket.Upgrader{
					CheckOrigin: func(r *http.Request) bool { return true },
				}
				targetURL := "ws://" + p.Remotes[k].remote.Host + r.WithContext(ctx).RequestURI
				backendConn, _, err := websocket.DefaultDialer.Dial(targetURL, reqHeader)
				if err != nil {
					log.Println("Backend dial error:", err)
					return
				}
				defer backendConn.Close()

				// Upgrade client connection
				backendSubprotocol := backendConn.Subprotocol()
				log.Println("backendSubprotocol", backendSubprotocol)
				var clientConn *websocket.Conn

				if len(backendSubprotocol) > 0 {
					clientConn, err = upgrader.Upgrade(w, r, http.Header{
						"sec-websocket-protocol": []string{backendSubprotocol},
					})
				} else {
					clientConn, err = upgrader.Upgrade(w, r, nil)
				}
				if err != nil {
					log.Println("Client upgrade error:", err)
					return
				}
				defer clientConn.Close()

				log.Println("WebSocket proxy connected")
				proxyCopy := func(src, dst *websocket.Conn, errCh chan error) {
					for {
						msgType, msg, err := src.ReadMessage()
						if err != nil {
							errCh <- err
							return
						}
						err = dst.WriteMessage(msgType, msg)
						if err != nil {
							errCh <- err
							return
						}
					}
				}
				// Proxy messages in both directions
				errCh := make(chan error, 2)
				go proxyCopy(clientConn, backendConn, errCh)
				go proxyCopy(backendConn, clientConn, errCh)
				<-errCh
			} else {
				p.Remotes[k].reverse.ServeHTTP(w, r.WithContext(ctx))
			}
			return
		}
	}
}
func (p *RestEndpointDefinition) Register() {
	p.Remotes = make(map[string]remote)
	if p.Proxies != nil {
		for _, u := range p.Proxies {
			r, err := url.Parse(u)
			if err != nil {
				panic(err)
			}
			proxy := httputil.NewSingleHostReverseProxy(r)
			proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
				log.Println(e)
				p.serveNextProxy(false, w, r)
			}
			p.Remotes[u] = remote{remote: r, reverse: proxy}
		}
	}
}

func (p *RestEndpointDefinition) ServeProxy(c *gin.Context) {
	ar := availableRemote{all: make(map[string]bool)}
	index := 0
	for k, _ := range p.Remotes {
		if index == 0 {
			ar.current = k
		}
		index++
		ar.all[k] = true
	}
	ctx := context.WithValue(c.Request.Context(), availableRemoteId, &ar)
	ctx = context.WithValue(ctx, preHandlerCalled, false)
	p.serveNextProxy(true, c.Writer, c.Request.WithContext(ctx))
}

func HandleRequest(userName string, routeId string, c *gin.Context, endpoint *RestEndpointDefinition) {
	// TODO remove orphans
	loginDir := fmt.Sprintf("/tmp/.logins/%s", userName)
	if _, err := os.Stat(loginDir); os.IsNotExist(err) {
		debug := os.Getenv("GOLANG_DEBUG")
		var cmd *exec.Cmd
		if debug == "" {
			cmd = exec.Command("/etc/units/user.login.sh", userName)
		} else {
			cmd = exec.Command("podman", "exec", "-it", "rdev", "/etc/units/user.login.sh", userName)
		}
		if out, err := cmd.Output(); err != nil {
			log.Println(err)
			return
		} else {
			log.Println(string(out))
		}
	}

	id := fmt.Sprintf("%s_%s", userName, routeId)
	if AllRestEndpoint[id] != nil {
		log.Println("handle logged in user", id)
		AllRestEndpoint[id].ServeProxy(c)
		return
	}
	content, err := os.ReadFile(fmt.Sprintf("%s/%s.port", loginDir, routeId))
	if err == nil {
		AllRestEndpoint[id] = endpoint
		endpoint.Id = id
		endpoint.Proxies = []string{fmt.Sprintf("http://localhost:%s", string(content))}
		endpoint.Register()
		log.Println("register and handle logged in user", id)
		endpoint.ServeProxy(c)
	}
}

func init() {
	flag.BoolVar(&Config.UseSAMLAuth, "saml", false, "Use saml auth(default is dummy) ")

	flag.IntVar(&Config.Port, "port", 10111, "Port(10111)")
	flag.StringVar(&Config.SimpleAuthTemplatePath, "simple_auth_template_path", "simple/index.html", "")
	flag.StringVar(&Config.SAML.CookieName, "cookie_name", "remove-dev-env", "")

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

	log.Println("SimpleAuthTemplatePath", Config.SimpleAuthTemplatePath)

	if Config.UseSAMLAuth {
		saml.InitSAML()
	}
}

func main() {
	AllRoutesRegexp = append(AllRoutesRegexp, &RouteMatch{Regex: regexp.MustCompile(`^code.`), Id: "CODE"})
	AllRoutesRegexp = append(AllRoutesRegexp, &RouteMatch{Regex: regexp.MustCompile(`^rsh.`), Id: "RSH"})

	r := gin.Default()
	r.LoadHTMLFiles(Config.SimpleAuthTemplatePath)
	r.NoRoute(func(c *gin.Context) {
		cookie, err := c.Request.Cookie(Config.SAML.CookieName)
		host := c.Request.Host
		log.Println("Handle host", host)
		if err != nil {
			log.Println("Handle anonymous user")
			if Config.UseSAMLAuth {
				if strings.HasPrefix(c.Request.URL.Path, "/saml/") {
					log.Println("SAMLSP handel /saml/")
					SAMLSP.ServeHTTP(c.Writer, c.Request)
				} else {
					_, err := SAMLSP.Session.GetSession(c.Request)
					log.Println("SAMLSP err >", err)
					if err == samlsp.ErrNoSession {
						log.Println("SAMLSP HandleStartAuthFlow")
						SAMLSP.HandleStartAuthFlow(c.Writer, c.Request)
					}
				}
			} else {
				if strings.HasPrefix(c.Request.URL.Path, "/saml/") {
					var req simple.JWTUser
					if err := c.ShouldBindJSON(&req); err != nil {
						c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
						return
					}
					cookie, err := simple.Encode(req)
					if err == nil {
						c.SetCookie(
							Config.SAML.CookieName, // name
							cookie,                 // value
							3600,                   // maxAge (seconds)
							"/",                    // path
							".localhost.com",       // domain
							false,                  // secure
							true,                   // httpOnly
						)
						c.JSON(200,
							gin.H{
								"status": "success",
							})

					} else {
						c.JSON(500,
							gin.H{
								"status": "failed",
							})
					}
				} else {
					c.HTML(200, "index.html", gin.H{
						"title": "Welcome to Gin",
					})
				}
			}
		} else {
			user, err := simple.Decode(cookie.Value)
			if err == nil {
				found := false
				for _, route := range AllRoutesRegexp {
					found = route.Regex.MatchString(host)
					if found {
						HandleRequest(user.Name, route.Id, c, &RestEndpointDefinition{
							PreHandler: func(ep *RestEndpointDefinition, w http.ResponseWriter, r *http.Request) {
								w.Header().Add("foo", "bar")
							}})
						break
					}
				}
				if !found {
					log.Println("Handle logged in user", cookie)
					return
				}
			}
		}
	})

	if len(Config.CertFile) > 0 && len(Config.KeyFile) > 0 {
		if err := r.RunTLS(fmt.Sprintf(":%d", Config.Port), Config.CertFile, Config.KeyFile); err != nil {
			log.Fatal("Failed to run HTTPS server: ", err)
		}
	} else {
		r.Run(fmt.Sprintf(":%d", Config.Port))
	}

}
