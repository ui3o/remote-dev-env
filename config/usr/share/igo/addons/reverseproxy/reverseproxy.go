package main

import (
	"io"
	"log"
	"net/http"
	"regexp"

	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/ui3o/remote-dev-env/reverseproxy/saml"
	"github.com/ui3o/remote-dev-env/reverseproxy/simple"
)

const (
	availableRemoteId = "availableRemoteId"
	preHandlerCalled  = "preHandlerCalled"
)

type AllRestEndpointDefinition struct {
	Endpoints map[string]*RestEndpointDefinition
}
type RuntimeConfig struct {
	CookieName               string
	CookieAge                int
	Port                     int
	CertFile                 string
	KeyFile                  string
	TemplateRootPath         string
	LocalGlobalPortList      string
	ReplaceSubdomainToCookie bool
	UseSAMLAuth              bool
	SAML                     *saml.SAMLConfig
}

type RouteMatch struct {
	Regex      *regexp.Regexp
	Id         string
	PreHandler func(ep *RestEndpointDefinition, c *gin.Context)
}

type RestEndpointDefinition struct {
	RouteId    string
	UserName   string
	RemoteUrls []string
	Remotes    map[string]*url.URL
}

type availableRemote struct {
	current string
	all     map[string]bool
}

func (p *RestEndpointDefinition) serveHTTPRequest(user *simple.JWTUser, target string, c *gin.Context) {
	// Create the new request to the backend
	req, err := http.NewRequest(c.Request.Method, "http://"+target+c.Request.RequestURI, c.Request.Body)
	if err != nil {
		if !p.tryNextProxyBackend(user, false, c) {
			c.String(http.StatusInternalServerError, "Failed to create request: %v", err)
		}
		return
	}
	req.Header = c.Request.Header.Clone()

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		if !p.tryNextProxyBackend(user, false, c) {
			c.String(http.StatusBadGateway, "Failed to reach backend: %v", err)
		}
		return
	}
	defer resp.Body.Close()

	// Copy all headers
	for k, v := range resp.Header {
		for _, vv := range v {
			c.Writer.Header().Add(k, vv)
		}
	}
	c.Status(resp.StatusCode)

	// Copy body
	accept := c.Request.Header.Get("Accept")

	if strings.Contains(accept, "text/html") {
		log.Println(debugHeader(user.Name), "start to replace response body")

		if bodyBytes, err := io.ReadAll(resp.Body); err != nil {
			log.Println(debugHeader(user.Name), "Failed to read response body: %v", err)
		} else {
			// Enable CORS for the injected script
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "*")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "*")
			scheme := "http"
			if c.Request.TLS != nil {
				scheme = "https"
			}
			// Replace first subdomain with "admin"
			hostParts := strings.SplitN(user.Host, ".", 2)
			adminHost := "admin"
			if len(hostParts) == 2 {
				adminHost = "admin." + hostParts[1]
			}
			src := scheme + "://" + adminHost + "/static/inject.js"
			log.Println(debugHeader(user.Name), "add this path to script:", src)

			bodyStr := string(bodyBytes)
			bodyStr = strings.Replace(bodyStr, "</body>", `<script src="`+src+`"></script></body>`, 1)
			c.Writer.Write([]byte(bodyStr))
			return
		}
	}
	io.Copy(c.Writer, resp.Body)
}

func (p *RestEndpointDefinition) serveWebsocket(remoteUrl string, c *gin.Context) {
	// Handle WebSocket upgrade
	var err error
	host := p.Remotes[remoteUrl].Host
	reqHeader := http.Header{
		"host": []string{host},
	}
	subProtocols := c.Request.Header.Get("sec-websocket-protocol")
	if len(subProtocols) > 0 {
		reqHeader["sec-websocket-protocol"] = []string{subProtocols}
	}
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	targetURL := "ws://" + host + c.Request.RequestURI
	backendConn, _, err := websocket.DefaultDialer.Dial(targetURL, reqHeader)
	if err != nil {
		log.Println(debugHeader(p.UserName), "Backend dial error:", err)
		return
	}
	defer backendConn.Close()

	// Upgrade client connection
	backendSubprotocol := backendConn.Subprotocol()
	log.Println(debugHeader(p.UserName), "backendSubprotocol", backendSubprotocol)
	var clientConn *websocket.Conn

	if len(backendSubprotocol) > 0 {
		clientConn, err = upgrader.Upgrade(c.Writer, c.Request, http.Header{
			"sec-websocket-protocol": []string{backendSubprotocol},
		})
	} else {
		clientConn, err = upgrader.Upgrade(c.Writer, c.Request, nil)
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
}

func (p *RestEndpointDefinition) tryNextProxyBackend(user *simple.JWTUser, currentState bool, c *gin.Context) bool {
	ar := c.MustGet(availableRemoteId).(*availableRemote)
	pHandlerCall := c.MustGet(preHandlerCalled).(bool)
	ar.all[ar.current] = currentState
	for k, v := range ar.all {
		if v {
			ar.current = k

			c.Request.URL.Host = p.Remotes[k].Host
			c.Request.Host = p.Remotes[k].Host
			c.Request.Header.Set("X-Forwarded-Host", c.Request.Host)
			c.Request.Header.Set("X-Forwarded-For", c.Request.RemoteAddr)
			routeId := c.Request.Header.Get(REQ_HEADER_ROUTE_ID)

			if !pHandlerCall && AllRoutesRegexp[routeId] != nil &&
				AllRoutesRegexp[routeId].PreHandler != nil {
				log.Println("PreHandler for ", p.UserName, p.RouteId)
				AllRoutesRegexp[routeId].PreHandler(p, c)
			}
			log.Println(debugHeader(p.UserName), "serveNextProxy [", c.Request.Host, "]")

			c.Set(preHandlerCalled, true)
			if strings.ToLower(c.Request.Header.Get("Connection")) == "upgrade" &&
				strings.ToLower(c.Request.Header.Get("Upgrade")) == "websocket" {
				p.serveWebsocket(k, c)
			} else {
				p.serveHTTPRequest(user, p.Remotes[k].Host, c)
			}
			return true
		} else {
			log.Println(debugHeader(p.UserName), "Remote", k, "is not available, skip")
		}
	}
	log.Println(debugHeader(p.UserName), "No more remote", p.RouteId, "is available!!!")
	return false
}

func (p *RestEndpointDefinition) Register() {
	p.Remotes = make(map[string]*url.URL)
	if p.RemoteUrls != nil {
		for _, u := range p.RemoteUrls {
			if url, err := url.Parse(u); err != nil {
				panic(err)
			} else {
				p.Remotes[u] = url
			}
		}
	}
}

func (p *RestEndpointDefinition) StartServeProxy(user *simple.JWTUser, c *gin.Context) {
	ar := availableRemote{all: make(map[string]bool)}
	index := 0
	for k := range p.Remotes {
		if index == 0 {
			ar.current = k
		}
		index++
		ar.all[k] = true
	}

	c.Set(availableRemoteId, &ar)
	c.Set(preHandlerCalled, false)
	p.tryNextProxyBackend(user, true, c)
}

func HandleRequest(user *simple.JWTUser, c *gin.Context) {
	checkUserRouteId(c)
	log.Println(debugHeader(user.Name), "handle logged in user and route", user.RouteId)
	AllRestEndpoint[user.Name].Endpoints[user.RouteId].StartServeProxy(user, c)
}

func createCookie(c *gin.Context, user *simple.JWTUser, cookieName, cookieData string) {
	host := user.Host
	log.Println(debugHeader(user.Name), "c.SetCookie >", cookieName)
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
	c.SetCookie(cookieName, cookieData, Config.CookieAge, "/", domain, false, true)
}

func findRoute(user *simple.JWTUser, c *gin.Context) {
	for _, route := range AllRoutesRegexp {
		found := route.Regex.MatchString(user.Host)
		if found {
			user.RouteId = route.Id
			c.Request.Header.Add(REQ_HEADER_ROUTE_ID, route.Id)
			return
		}
	}
	log.Println(debugHeader(user.Name), "findRoute can not resolve route!!!")

}
