package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ui3o/remote-dev-env/igo-reverseproxy/saml"
	"github.com/ui3o/remote-dev-env/igo-reverseproxy/simple"
)

func checkPortIsOpened(userName, port string) {
	for {
		cmd := exec.Command("nc", "-z", "localhost", port)
		err := cmd.Run()
		if err == nil {
			log.Println(debugHeader(userName), "Port is opened: ", port)
			break
		} else {
			log.Println(debugHeader(userName), "Port is not available yet for:", port)
			time.Sleep(150 * time.Microsecond)
		}
	}
}

func watchContainerRunning(userName, routeId string) {
	go func() {
		cmd := exec.Command("pake", "listenContainerRunning.10", userName)
		cmd.Run()
		log.Println(debugHeader(userName), "Remove ", routeId, " from AllRestEndpoint.Endpoint")
		delete(AllRestEndpoint[userName].Endpoints, routeId)
	}()
}

func runCmd(userName, name string, arg ...string) (string, error) {
	cmd := exec.Command(name, arg...)
	log.Println(debugHeader(userName), "execute command", name, arg)
	if out, err := cmd.Output(); err != nil {
		log.Println(debugHeader(userName), "execute", name, arg, " has error:", err)
		return "", err
	} else {
		log.Println(debugHeader(userName), "execute", name, arg, " out:", string(out))
		return string(out), nil
	}
}

func modifyAccessFile(c *gin.Context, userName string) {
	filePath := fmt.Sprintf("/tmp/.runtime/logins/%s/.access", userName)
	if err := os.MkdirAll(fmt.Sprintf("/tmp/.runtime/logins/%s", userName), 0755); err != nil {
		log.Println(debugHeader(userName), "Failed to create directory:", err)
		return
	}
	f, err := os.Create(filePath)
	if err != nil {
		log.Println(debugHeader(userName), "Failed to create file:", err)
		return
	} else {
		f.Close()
	}
	_ = os.Chtimes(filePath, time.Now(), time.Now())
}

func checkUserRouteId(c *gin.Context) {
	userName := c.GetHeader(REQ_HEADER_PROXY_USER_NAME)
	routeId := c.GetHeader(REQ_HEADER_ROUTE_ID)
	if AllRestEndpoint[userName] == nil || AllRestEndpoint[userName].Endpoints[routeId] == nil {
		runCmd(userName, "pake", "start.10", userName)
		if out, err := runCmd(userName, "pake", "getPortForRouteID.20", userName, routeId); err == nil {
			checkPortIsOpened(userName, out)
			watchContainerRunning(userName, routeId)
			if AllRestEndpoint[userName] == nil {
				AllRestEndpoint[userName] = &AllRestEndpointDefinition{}
				AllRestEndpoint[userName].Endpoints = make(map[string]*RestEndpointDefinition)
			}
			AllRestEndpoint[userName].Endpoints[routeId] = &RestEndpointDefinition{
				RouteId:    routeId,
				UserName:   userName,
				RemoteUrls: []string{fmt.Sprintf("http://localhost:%s", out)},
			}
			AllRestEndpoint[userName].Endpoints[routeId].Register()
		}
	}
}

func readUser(c *gin.Context) *simple.JWTUser {
	user := simple.JWTUser{
		Name: DEFAULT_USERNAME,
		Host: c.Request.Host,
	}
	if Config.ReplaceSubdomainToCookie {
		if domainCookie, err := c.Cookie(DOMAIN_COOKIE_NAME); err == nil {
			user.Host = domainCookie
		}
	}
	log.Println("[NONE] readUser for host(", user.Host, ")")

	if Config.UseSAMLAuth {
		log.Println("[NONE] readUser Config.UseSAMLAuth start")
		if session, err := SAMLSP.Session.GetSession(c.Request); err == nil {
			if cookieSession, ok := session.(saml.JWTSessionClaims); ok {
				domainAndName := cookieSession.StandardClaims.Subject
				u := strings.Split(domainAndName, "\\")
				user.IsValid = true
				user.Name = strings.ToLower(saml.Pop(&u))
				user.Domain = strings.ToLower(saml.Pop(&u))
				user.Email = strings.ToLower(cookieSession.Attributes.Get("emailaddress"))
			} else {
				log.Println("[NONE] readUser JWTSessionClaims cast error")
			}
		} else {
			log.Println("[NONE] readUser session error: ", err)
		}
	} else {
		if cookie, err := c.Cookie(Config.CookieName); err == nil {
			if u, err := simple.Decode(cookie); err == nil {
				user.IsValid = true
				user.Name = u.Name
				user.Domain = u.Domain
				user.Email = u.Email
			} else {
				log.Println("[NONE] readUser simple.Decode error:", err)
			}
		} else {
			log.Println("[NONE] readUser cookie get error:", err)
		}
	}

	if user.IsValid {
		c.Request.Header.Add(REQ_HEADER_PROXY_USER_NAME, user.Name)
	}
	findRoute(&user, c)
	log.Println(debugHeader(user.Name), "final readUser result", user.ToString())
	return &user
}
