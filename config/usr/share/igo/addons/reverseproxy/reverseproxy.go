package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"regexp"
	"time"

	"net/url"
	"strings"

	"github.com/crewjam/saml/samlsp"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/websocket"
	"github.com/ui3o/remote-dev-env/igo-reverseproxy/saml"
	"github.com/ui3o/remote-dev-env/igo-reverseproxy/simple"
	"go.senan.xyz/flagconf"
)

type key int

const (
	availableRemoteId key = iota
	preHandlerCalled
)

var (
	SAMLSP          *samlsp.Middleware
	AllRestEndpoint = make(map[string]*AllRestEndpointDefinition)
	AllRoutesRegexp = []*RouteMatch{}
	Config          = RuntimeConfig{
		SAML: &saml.SAMLConf,
	}
)

type AllRestEndpointDefinition struct {
	Endpoint     map[string]*RestEndpointDefinition
	LastActivity time.Time
}
type RuntimeConfig struct {
	CookieName               string
	CookieAge                int
	Port                     int
	CertFile                 string
	KeyFile                  string
	SimpleAuthTemplatePath   string
	LocalstorageTemplatePath string
	ReplaceSubdomainToCookie bool
	UseSAMLAuth              bool
	SAML                     *saml.SAMLConfig
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
	RouteId    string
	UserName   string
	Proxies    []string
	Remotes    map[string]remote
	PreHandler func(ep *RestEndpointDefinition, w http.ResponseWriter, r *http.Request)
}

type availableRemote struct {
	current string
	all     map[string]bool
}

type UserEnv struct {
	Storage string `json:"storage"`
	jwt.RegisteredClaims
}

const DEFAULT_USERNAME = "zzz"
const DOMAIN_COOKIE_NAME = "remote-dev-domain"
const LOCALSTORAGE_COOKIE_NAME = "remote-dev-localstorage"
const PROXY_USER_NAME = "proxy-user-name"

func debugHeader(username string) string {
	return fmt.Sprintf("[%s] ", username)
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
				log.Println("PreHandler for ", p.UserName, p.RouteId)
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
					loginUser(r, true)
					log.Println(debugHeader(p.UserName), "Backend dial error:", err)
					return
				}
				defer backendConn.Close()

				// Upgrade client connection
				backendSubprotocol := backendConn.Subprotocol()
				log.Println(debugHeader(p.UserName), "backendSubprotocol", backendSubprotocol)
				var clientConn *websocket.Conn

				if len(backendSubprotocol) > 0 {
					clientConn, err = upgrader.Upgrade(w, r, http.Header{
						"sec-websocket-protocol": []string{backendSubprotocol},
					})
				} else {
					clientConn, err = upgrader.Upgrade(w, r, nil)
				}
				if err != nil {
					log.Println(debugHeader(p.UserName), "Client upgrade error:", err)
					return
				}
				defer clientConn.Close()

				log.Println(debugHeader(p.UserName), "WebSocket proxy connected")
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
		} else {
			loginUser(r, true)
			log.Println(debugHeader(p.UserName), "Remote", k, "is not available, skip")
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
				log.Println(debugHeader(p.UserName), "ErrorHandler has error", e)
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

func loginUser(r *http.Request, force bool) {
	userName := r.Header.Get(PROXY_USER_NAME)
	loginDir := fmt.Sprintf("/tmp/.runtime/logins/%s", userName)
	if _, err := os.Stat(loginDir); os.IsNotExist(err) || force {
		log.Println(debugHeader(userName), "run pake start")
		cmd := exec.Command("pake", "start.10", userName)
		log.Println(debugHeader(userName), "execute command and grab output")
		if out, err := cmd.Output(); err != nil {
			log.Println(debugHeader(userName), "execute has error:", err)
			return
		} else {
			log.Println(debugHeader(userName), "out:", string(out))
		}
	}
}

func removeLogoutUsers() {
	// remove logged out user from reverse proxy
	for k := range AllRestEndpoint {
		if AllRestEndpoint[k].LastActivity.Before(time.Now().Add(-3 * time.Second)) {
			loginDir := fmt.Sprintf("/tmp/.runtime/logins/%s/.running", k)
			if _, err := os.Stat(loginDir); os.IsNotExist(err) {
				log.Println(debugHeader(k), "Remove user from AllRestEndpoint")
				delete(AllRestEndpoint, k)
			}
		}
	}

	if dirs, err := os.ReadDir("/tmp/.runtime/logins"); err == nil {
		for _, dir := range dirs {
			if dir.IsDir() {
				userName := dir.Name()
				if AllRestEndpoint[userName] == nil || AllRestEndpoint[userName].LastActivity.Before(time.Now().Add(-3*time.Second)) {
					runningStatus := fmt.Sprintf("/tmp/.runtime/logins/%s/.running", userName)
					if _, err := os.Stat(runningStatus); os.IsNotExist(err) {
						log.Println(debugHeader(userName), "Remove user folder", runningStatus)
						os.RemoveAll(fmt.Sprintf("/tmp/.runtime/logins/%s", userName))
					} else {
						os.RemoveAll(runningStatus)
					}
				}
			}
		}
	}

}

func HandleRequest(userName string, routeId string, c *gin.Context, endpoint *RestEndpointDefinition) {
	loginDir := fmt.Sprintf("/tmp/.runtime/logins/%s", userName)
	if AllRestEndpoint[userName] != nil && AllRestEndpoint[userName].Endpoint[routeId] != nil {
		log.Println(debugHeader(userName), "handle logged in user and route", routeId)
		AllRestEndpoint[userName].LastActivity = time.Now()
		AllRestEndpoint[userName].Endpoint[routeId].ServeProxy(c)
		return
	}
	content, err := os.ReadFile(fmt.Sprintf("%s/%s.port", loginDir, routeId))
	if err == nil {
		AllRestEndpoint[userName] = &AllRestEndpointDefinition{}
		AllRestEndpoint[userName].Endpoint = make(map[string]*RestEndpointDefinition)
		AllRestEndpoint[userName].Endpoint[routeId] = endpoint
		endpoint.RouteId = routeId
		endpoint.UserName = userName
		endpoint.Proxies = []string{fmt.Sprintf("http://localhost:%s", string(content))}
		endpoint.Register()
		log.Println(debugHeader(userName), "register and handle logged in user and route", routeId)
		AllRestEndpoint[userName].LastActivity = time.Now()
		endpoint.ServeProxy(c)
	} else {
		log.Println(debugHeader(userName), "HandleRequest has an error", err)
	}
}

func init() {
	flag.CommandLine.Init("env_param_reverseproxy", flag.ExitOnError)

	flag.BoolVar(&Config.UseSAMLAuth, "saml", false, "Use saml auth(default is dummy) ")
	flag.BoolVar(&Config.ReplaceSubdomainToCookie, "replace_subdomain_to_cookie", false, "Use saml auth(default is dummy) ")

	flag.IntVar(&Config.Port, "port", 10111, "Port(10111)")
	flag.IntVar(&Config.CookieAge, "age", 3600, "cookie age in sec")
	flag.StringVar(&Config.SimpleAuthTemplatePath, "simple_auth_template_path", "simple/auth.html", "")
	flag.StringVar(&Config.LocalstorageTemplatePath, "localstorage_template_path", "localstorage/localstorage.html", "")

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

	log.Println("[INIT] SimpleAuthTemplatePath", Config.SimpleAuthTemplatePath)

	if Config.UseSAMLAuth {
		if saml, err := saml.InitSAML(); err == nil {
			SAMLSP = saml
		} else {
			log.Println("[INIT] Error Init SAMLSP is >", err)
		}
	}
}

func createCookie(r *http.Request, w http.ResponseWriter, userName, cookieName, cookieData string) {
	host := r.Host
	log.Println(debugHeader(userName), "c.SetCookie >", cookieName)
	if conditions := strings.Split(host, ":"); len(conditions) > 0 {
		host = conditions[0]
	}
	domain := host
	// parentDomain := host
	if conditions := strings.Split(host, "."); len(conditions) > 2 {
		// ".localhost.com"
		conditions = append(conditions[:0], conditions[1:]...)
		domain = "." + strings.Join(conditions, ".")
		// parentDomain = strings.Join(conditions, ".")
	}
	w.Header().Add("Set-Cookie", fmt.Sprintf("%s=%s; Max-Age=%d; Path=/; Domain=%s; Secure=false; HttpOnly=true", cookieName, cookieData, Config.CookieAge, domain))
}

func findRoute(host string) (found bool, foundedRoute *RouteMatch) {
	for _, route := range AllRoutesRegexp {
		found := route.Regex.MatchString(host)
		if found {
			foundedRoute = route
			return true, route
		}
	}
	return false, nil
}

func modifyAccessFile(r *http.Request, userName string) {
	removeLogoutUsers()
	filePath := fmt.Sprintf("/tmp/.runtime/logins/%s/.access", userName)
	f, err := os.Create(filePath)
	if err != nil {
		log.Println(debugHeader(userName), "Failed to create file:", err)
		loginUser(r, false)
		return
	} else {
		f.Close()
	}
	_ = os.Chtimes(filePath, time.Now(), time.Now())
}

func createSecretFile(userName string) {
	cmd := exec.Command("pake", "store_secret.10", userName)
	log.Println(debugHeader(userName), "createSecretFile execute command and grab output")
	if out, err := cmd.Output(); err != nil {
		log.Println(debugHeader(userName), "createSecretFile has error:", err)
		return
	} else {
		log.Println(debugHeader(userName), "out:", string(out))
	}

}

func checkLocalStorage(r *http.Request, user *simple.JWTUser) {
	filePath := fmt.Sprintf("/tmp/.runtime/logins/%s/localstorage.synced", user.Name)
	r.Header.Add(PROXY_USER_NAME, user.Name)
	_, localstorageErr := os.Stat(filePath)
	if _, err := r.Cookie(LOCALSTORAGE_COOKIE_NAME); err == nil && localstorageErr == nil {
		user.HasSecret = true
	}
}

func readUser(r *http.Request, w http.ResponseWriter) *simple.JWTUser {
	user := simple.JWTUser{
		Name: DEFAULT_USERNAME,
	}
	host := r.Host
	found, _ := findRoute(r.Host)
	if found {
		createCookie(r, w, DEFAULT_USERNAME, DOMAIN_COOKIE_NAME, r.Host)
	}

	if Config.ReplaceSubdomainToCookie {
		if domainCookie, err := r.Cookie(DOMAIN_COOKIE_NAME); err == nil {
			host = domainCookie.Value
		}
	}

	log.Println("[NONE] readUser for host(", host, "), path(", host, ")")
	if strings.Contains(r.URL.Path, "/static/out/vs/workbench") || strings.Contains(r.URL.Path, "/_static/src/browser") || strings.Contains(r.URL.Path, "/_static/out/browser") {
		virtualUserName := DEFAULT_USERNAME
		if Config.ReplaceSubdomainToCookie {
			host = "code." + host
		}
		if dirs, err := os.ReadDir("/tmp/.runtime/logins"); err == nil {
			for _, dir := range dirs {
				if dir.IsDir() {
					virtualUserName = dir.Name()
					break
				}
			}
		}
		log.Println("[NONE] readUser virtual user create", r.URL.Path, virtualUserName)
		r.Header.Add(PROXY_USER_NAME, virtualUserName)
		user.IsValid = true
		user.HasSecret = true
		user.Host = host
		user.Name = virtualUserName
		user.Domain = virtualUserName
		user.Email = virtualUserName
	}

	if Config.UseSAMLAuth {
		log.Println("[NONE] readUser Config.UseSAMLAuth start")
		if session, err := SAMLSP.Session.GetSession(r); err == nil {
			if cookieSession, ok := session.(saml.JWTSessionClaims); ok {
				domainAndName := cookieSession.StandardClaims.Subject
				u := strings.Split(domainAndName, "\\")
				user.IsValid = true
				user.Host = host
				user.Name = strings.ToLower(saml.Pop(&u))
				user.Domain = strings.ToLower(saml.Pop(&u))
				user.Email = strings.ToLower(cookieSession.Attributes.Get("emailaddress"))
				checkLocalStorage(r, &user)
			} else {
				log.Println("[NONE] readUser JWTSessionClaims cast error")
			}
		} else {
			log.Println("[NONE] readUser session error: ", err)
		}
	} else {
		if cookie, err := r.Cookie(Config.CookieName); err == nil {
			if u, err := simple.Decode(cookie.Value); err == nil {
				user.IsValid = true
				user.Host = host
				user.Name = u.Name
				user.Domain = u.Domain
				user.Email = u.Email
				checkLocalStorage(r, &user)
			} else {
				log.Println("[NONE] readUser simple.Decode error:", err)
			}
		} else {
			log.Println("[NONE] readUser cookie get error:", err)
		}
	}
	log.Println(debugHeader(user.Name), "final readUser result", user.ToString())
	return &user
}

func main() {
	AllRoutesRegexp = append(AllRoutesRegexp, &RouteMatch{Regex: regexp.MustCompile(`^code.`), Id: "CODE"})
	AllRoutesRegexp = append(AllRoutesRegexp, &RouteMatch{Regex: regexp.MustCompile(`^rsh.`), Id: "RSH"})

	r := gin.Default()
	r.LoadHTMLFiles(Config.SimpleAuthTemplatePath, Config.LocalstorageTemplatePath)
	r.NoRoute(func(c *gin.Context) {
		log.Println("[REQ_START] Handle request => |", c.Request.Host, "|", c.Request.URL.Path, "|")

		if strings.Contains(c.Request.URL.Path, ".js") {
			c.Header("Content-Type", "application/javascript")
		}

		if user := readUser(c.Request, c.Writer); !user.IsValid {
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
						createCookie(c.Request, c.Writer, user.Name, Config.CookieName, cookie)
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
			modifyAccessFile(c.Request, user.Name)
			// load localstorage page
			if !user.HasSecret {
				if c.Request.URL.Path == "/remote-dev-localstorage-loader" {
					var req UserEnv
					if err := c.ShouldBindJSON(&req); err != nil {
						log.Println(debugHeader(user.Name), "Catch ShouldBindJSON err >", err)
						c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
						return
					}
					log.Println(debugHeader(user.Name), "localstorageCookie is", req.Storage)
					createCookie(c.Request, c.Writer, user.Name, LOCALSTORAGE_COOKIE_NAME, "synced")

					filePath := fmt.Sprintf("/tmp/.runtime/logins/%s/localstorage", user.Name)
					if err := os.WriteFile(filePath, []byte(req.Storage), 0600); err != nil {
						log.Println(debugHeader(user.Name), "Failed to write localstorage file:", err)
					} else {
						log.Println(debugHeader(user.Name), "have to set storage into secret store for", user.Name)
						createSecretFile(user.Name)
					}
					c.JSON(200,
						gin.H{
							"status": "success",
						})
					return
				} else {
					log.Println(debugHeader(user.Name), "load localstorage.html")
					c.HTML(200, "localstorage.html", gin.H{
						"title": "Send localstorage to server",
					})

				}
				return
			} else {
				log.Println(debugHeader(user.Name), "Handle logged in user and start to findRoute")
				if found, foundedRoute := findRoute(user.Host); found {
					log.Println(debugHeader(user.Name), "findRoute has found a route")
					HandleRequest(user.Name, foundedRoute.Id, c, &RestEndpointDefinition{
						PreHandler: func(ep *RestEndpointDefinition, w http.ResponseWriter, r *http.Request) {
							// w.Header().Add("foo", "bar")
						}})
				} else {
					log.Println(debugHeader(user.Name), "Handle logged in user")
					return
				}
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
