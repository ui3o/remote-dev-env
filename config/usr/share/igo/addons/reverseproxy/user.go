package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"time"

	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ui3o/codebox/reverseproxy/saml"
	"github.com/ui3o/codebox/reverseproxy/simple"
)

func checkPortIsOpened(userName, hostname, port string) error {
	for i := Config.MaxRetryCountForPortOpening; i > 0; i-- {
		cmd := exec.Command("nc", "-z", hostname, port)
		err := cmd.Run()
		if err == nil {
			log.Println(debugHeader(userName), "Port is opened: ", port)
			return nil
		} else {
			log.Println(debugHeader(userName), "Port is not available yet for:", port)
			time.Sleep(100 * time.Microsecond)
		}
	}
	return errors.New("port is not available after retries")
}

func watchContainerRunning(userName, routeId string) {
	go func() {
		cmd := exec.Command("pake", "listenContainerRunning", userName)
		cmd.Dir = Config.TemplateRootPath + "pake"
		cmd.Run()
		log.Println(debugHeader(userName), "Remove ", routeId, " from AllRestEndpoint.Endpoint")
		for _, globalName := range Config.GlobalPortList {
			AllGlobalEndpoints[globalName].UnRegister(userName)
			for uname := range AllRestEndpoint {
				AllRestEndpoint[uname].Endpoints[globalName].Remotes = AllGlobalEndpoints[globalName].Remotes
			}
		}
		delete(AllRestEndpoint[userName].Endpoints, routeId)
	}()
}

func runPake(userName, name string, arg ...string) (string, int) {
	cmd := exec.Command(name, arg...)
	cmd.Dir = Config.TemplateRootPath + "pake"
	log.Println(debugHeader(userName), "execute command", name, arg)
	if out, err := cmd.CombinedOutput(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			log.Println(debugHeader(userName), "execute", name, arg, "stderr:", string(out), " has error:", err.Error())
			return string(out), exitErr.ExitCode()
		} else {
			log.Println(debugHeader(userName), "execute", name, arg, " has error:", err, "stderr not detected")
			return string(out), 127
		}
	} else {
		log.Println(debugHeader(userName), "execute", name, arg, " out:", string(out))
		return string(out), 0
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

func userWhitelistWatcherInit() {
	log.Println("[INIT] userWhitelistWatcherInit started...")
	go func() {
		for {
			data, err := os.ReadFile(Config.UserWhitelistConfigPath)
			if err != nil {
				log.Println("[WHITELIST] Failed to read whitelist config:", err)
			}
			Config.UserWhiteList = make(map[string]bool)
			err = json.Unmarshal(data, &Config.UserWhiteList)
			if err != nil {
				log.Println("[WHITELIST] Failed to parse whitelist config:", err)
			}
			time.Sleep(time.Duration(3) * time.Second)
		}
	}()
}

func userContainerRemoverInit() {
	go func() {
		for {
			log.Println("[REMOVER] userContainerRemoverInit running...")
			runPake("REMOVER", "pake", "removeIdleUsers", fmt.Sprintf("%d", Config.UserIdleKillAfterTimeout))
			time.Sleep(time.Duration(Config.UserIdleCheckInterVal) * time.Minute)
		}
	}()
}

func userCreatorInit() {
	go func() {
		for c := range CreateUserChannel {
			userName := c.GetHeader(REQ_HEADER_PROXY_USER_NAME)
			userEmail := c.GetHeader(REQ_HEADER_PROXY_USER_EMAIL)
			routeId := c.GetHeader(REQ_HEADER_ROUTE_ID)

			log.Println(debugHeader(userName), "CREATOR received start")
			if err := os.MkdirAll(Config.HomeFolderPath+userName, 0755); err != nil {
				log.Println(debugHeader(userName), "Failed to create home directory for the user:", err)
			}
			globalPortStar, _ := runPake(userName, "pake", "getGlobalPortStart")
			globalPortStartNumber, _ := strconv.Atoi(strings.TrimSpace(globalPortStar))
			hostname, _ := runPake(userName, "pake", "getEndpointHostname", userName)
			runPake(userName, "pake", "start", userName, userEmail)
			var success error = errors.New("this endpoint not available at the moment, if you know it is available please refresh the page")
			if port, exitCode := runPake(userName, "pake", "getPortForRouteID", userName, routeId); exitCode == 0 {
				if AllRestEndpoint[userName] == nil {
					AllRestEndpoint[userName] = &AllRestEndpointDefinition{}
					AllRestEndpoint[userName].Endpoints = make(map[string]*RestEndpointDefinition)
					AllRestEndpoint[userName].Hostname = hostname
				}
				for pos, globalName := range Config.GlobalPortList {
					if AllGlobalEndpoints[globalName] == nil {
						AllGlobalEndpoints[globalName] = &RestEndpointDefinition{
							Remotes: make(map[string]*url.URL),
						}
					}
					for uname := range AllRestEndpoint {
						AllGlobalEndpoints[globalName].Register(AllRestEndpoint[uname].Hostname, strconv.Itoa(globalPortStartNumber+pos), uname)
					}
					for uname := range AllRestEndpoint {
						AllRestEndpoint[uname].Endpoints[globalName] = &RestEndpointDefinition{
							RouteId:  globalName,
							UserName: uname,
							Remotes:  AllGlobalEndpoints[globalName].Remotes,
						}
					}
				}
				if err := checkPortIsOpened(userName, hostname, port); err == nil {
					success = nil
					watchContainerRunning(userName, routeId)
					AllRestEndpoint[userName].Endpoints[routeId] = &RestEndpointDefinition{
						RouteId:  routeId,
						UserName: userName,
						Remotes:  make(map[string]*url.URL),
					}
					AllRestEndpoint[userName].Endpoints[routeId].Register(hostname, port, userName)
				}
			}
			if done, exists := c.Get(userCreationWaiter); exists {
				if ch, ok := done.(chan error); ok {
					ch <- success
					close(ch)
				}
			}
		}
	}()
}

func checkUserRouteId(c *gin.Context) error {
	userName := c.GetHeader(REQ_HEADER_PROXY_USER_NAME)
	routeId := c.GetHeader(REQ_HEADER_ROUTE_ID)
	if AllRestEndpoint[userName] == nil || AllRestEndpoint[userName].Endpoints[routeId] == nil {
		done := make(chan error, 1)
		// Wrap the context to include a done channel
		c.Set(userCreationWaiter, done)
		CreateUserChannel <- c
		portOpenSuccess := <-done
		if portOpenSuccess != nil {
			log.Println(debugHeader(userName), "user creation and port check has error")
			return portOpenSuccess
		} else {
			log.Println(debugHeader(userName), "user creation and port check done successfully")
		}
	}
	if htmlPage, exitCode := runPake(userName, "pake", "runUrlGuard", c.Request.RequestURI); exitCode == 9 {
		return errors.New(htmlPage)
	}
	return nil
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
		c.Request.Header.Add(REQ_HEADER_PROXY_USER_EMAIL, user.Email)
	}
	findRoute(&user, c)
	log.Println(debugHeader(user.Name), "final readUser result", user.ToString())
	return &user
}
