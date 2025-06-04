package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"

	"net/url"
	"regexp"
	"strings"

	"github.com/crewjam/saml/samlsp"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/ui3o/remote-dev-env/igo-reverseproxy/saml"
)

type key int

const (
	availableRemoteId key = iota
	preHandlerCalled
)

var (
	SAMLSP          *samlsp.Middleware
	AllRestEndpoint = []*RestEndpointDefinition{}
	Config          = RuntimeConfig{
		UseSAMLAuth: false,
		SAML:        &saml.SAMLConf,
	}
)

type RuntimeConfig struct {
	UseSAMLAuth bool
	SAML        *saml.SAMLConfig
}

type remote struct {
	remote  *url.URL
	reverse *httputil.ReverseProxy
}

type RestEndpointDefinition struct {
	Proxies    []string
	Regex      *regexp.Regexp
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
				log.Println("PreHandler for ", p.Regex.String())
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

func Register(endpoint *RestEndpointDefinition) *RestEndpointDefinition {
	AllRestEndpoint = append(AllRestEndpoint, endpoint)
	endpoint.Register()
	return endpoint
}

func FindRoute(host string, cookie *http.Cookie, c *gin.Context) bool {
	found := false
	for _, ep := range AllRestEndpoint {
		if ep.Regex != nil {
			found = ep.Regex.MatchString(host)
			if found {
				log.Println("Handle logged in user", cookie)
				ep.ServeProxy(c)
				break
			}
		}
	}
	return found
}

func init() {
	flag.BoolVar(&Config.UseSAMLAuth, "saml", false, "Use saml auth(default is dummy) ")

	flag.StringVar(&Config.SAML.IdpMetadataURL, "saml_idpmetadataurl", "", "")
	flag.StringVar(&Config.SAML.EntityID, "saml_entityid", "", "")
	flag.StringVar(&Config.SAML.CookieName, "saml_cookiename", "", "")
	flag.StringVar(&Config.SAML.RootURL, "saml_rooturl", "", "")
	flag.StringVar(&Config.SAML.CertFile, "saml_certfile", "", "")
	flag.StringVar(&Config.SAML.KeyFile, "saml_keyfile", "", "")
	flag.StringVar(&Config.SAML.Domain, "saml_domain", "", "")
	flag.StringVar(&Config.SAML.AuthnNameIDFormat, "saml_authnnameidformat", "", "")

	flag.Parse()

	if Config.UseSAMLAuth {
		saml.InitSAML()
	}
}

func main() {
	auth := Register(&RestEndpointDefinition{
		Proxies: []string{"http://localhost:9000"},
		PreHandler: func(ep *RestEndpointDefinition, w http.ResponseWriter, r *http.Request) {
			log.Println("PreHandler for auth")
		}})
	demo := Register(&RestEndpointDefinition{
		Proxies: []string{"http://localhost:9001", "http://localhost:9001"},
		PreHandler: func(ep *RestEndpointDefinition, w http.ResponseWriter, r *http.Request) {
			log.Println("PreHandler for demo")
			w.Header().Add("foo", "bar")
		}})

	Register(&RestEndpointDefinition{
		Regex:   regexp.MustCompile(`^code.`),
		Proxies: []string{"http://localhost:8080"},
		PreHandler: func(ep *RestEndpointDefinition, w http.ResponseWriter, r *http.Request) {
			w.Header().Add("foo", "bar")
		}})
	Register(&RestEndpointDefinition{
		Regex:   regexp.MustCompile(`^rsh.`),
		Proxies: []string{"http://localhost:7681"},
		PreHandler: func(ep *RestEndpointDefinition, w http.ResponseWriter, r *http.Request) {
			w.Header().Add("foo", "bar")
		}})
	r := gin.Default()

	r.NoRoute(func(c *gin.Context) {
		// TODO auto handle path
		cookie, err := c.Request.Cookie("remove-dev-env")
		host := c.Request.Host
		log.Println("Handle host", host)
		if err != nil {
			if Config.UseSAMLAuth {
				http.Handle("/saml/", SAMLSP)
				http.Handle("/", SAMLSP.RequireAccount(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					user := saml.UserFromRequest(r)
					fmt.Fprintf(w, "Hello, domain:%s, user:%s, email: %s!, Url:%s", user.Domain, user.Name, user.Email, r.URL)
				})))
			}
			log.Println("Handle anonymous user")
			auth.ServeProxy(c)
		} else {
			if !FindRoute(host, cookie, c) {
				log.Println("Handle logged in user", cookie)
				demo.ServeProxy(c)

			}
		}
	})
	r.Run(":10112")
}
