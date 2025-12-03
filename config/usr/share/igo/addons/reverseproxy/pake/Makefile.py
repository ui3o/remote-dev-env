import logging
import os
import subprocess
import gen_user_lock
import time

RCE_PATH = os.getenv("ENV_PARAM_REVERSEPROXY_RCE_PATH", "")
PODMAN_REMOTE = "-v r_dev_shared_runtime:/tmp/.runtime \
        -v /tmp/rce:/tmp/rce:ro \
        -v r_dev_shared_vol:/var/lib/shared-containers \
        localhost/local-codebox:latest"
CODEBOX_REMOTE_OPTS = os.getenv("CODEBOX_REMOTE_OPTS", PODMAN_REMOTE)

HOME_FOLDER_PATH = os.getenv("ENV_PARAM_REVERSEPROXY_HOME_FOLDER_PATH", "")

def podmanStart(developer="demo", email="demo@demo.com", portLock: int = 9000):
    # todo list all portRSH, portCODE
    p = f"\
        {RCE_PATH}rce podman run -d --rm --privileged --name codebox-user-{developer} --network host\
            -e DEVELOPER={developer}\
            -e CODEBOX_MODE_NO_REVERSEPROXY=true\
            --mount=type=bind,source=/etc/localtime,target=/etc/localtime,ro\
            -v {HOME_FOLDER_PATH}{developer}:/mine:Z \
            {gen_user_lock.createLabelList(developer, email, portLock)}\
            {CODEBOX_REMOTE_OPTS}\
        ".split(" ")
    return [arg for arg in p if arg]


def podmanCheckRun(developer="demo"):
    p = f"{RCE_PATH}rce podman container --filter=name=codebox-user-{developer} --format {{.Names}}".split(
        " "
    )
    return [arg for arg in p if arg]


def runningContainerList():
    p = [
        *f"\
        {RCE_PATH}rce podman ps --filter=name=codebox-user-.* --format {{{{.Labels.DEVELOPEREnv}}}}\
        ".split(" ")
    ]
    return [arg for arg in p if arg]


def portForRouteID(developer="demo", portRouteNameId: str = "NONE"):
    p = [
        *(
            f"{RCE_PATH}rce podman ps --filter=name=codebox-user-"
            + developer
            + " --format {{.Labels.port"
            + portRouteNameId
            + "Env}}"
            + "\
        "
        ).split(" ")
    ]
    return [arg for arg in p if arg]


def podmanWatchLogs(developer="demo"):
    p = [*(f"{RCE_PATH}rce podman attach codebox-user-{developer}").split(" ")]
    return [arg for arg in p if arg]

def removeGlobalPortLocks(developer="demo"):
    p = [*(f"rm -rf /tmp/.runtime/global_ports/{developer}").split(" ")]
    return [arg for arg in p if arg]


# this is a start function
def start(developer: str = "demo", email="demo@demo.com"):
    # todo set lock and all ports
    portLock: int = 10000
    logging.info(podmanStart(developer, email, portLock))
    subprocess.run(podmanStart(developer, email, portLock))


# this is a start function
def removeIdleUsers(idleTime: int = 1):
    out = subprocess.run(runningContainerList(), capture_output=True)
    result = out.stdout.decode().strip()
    if not result:
        logging.info("No running containers found.")
        return
    result = result.split("\n")
    for user in result:
        logging.info("checking idle user: %s", user)
        user = user.strip()
        access_file = f"/tmp/.runtime/logins/{user}/.access"
        try:
            stat = os.stat(access_file)
            last_access = stat.st_mtime
            idle_seconds = idleTime * 60
            if time.time() - last_access > idle_seconds:
                # remove the container if idle
                subprocess.run([f"{RCE_PATH}rce", "podman", "kill", f"codebox-user-{user}"])
        except FileNotFoundError:
            pass


# this function checks if the container is running and exit if not
def listenContainerRunning(developer: str = "demo"):
    logging.info(podmanWatchLogs(developer))
    subprocess.run(podmanWatchLogs(developer))
    logging.info(removeGlobalPortLocks(developer))
    subprocess.run(removeGlobalPortLocks(developer))


# this function returns container name for developer
def getEndpointHostname(developer: str = "demo"):
    print(f"codebox-user-{developer}", end="")

# this function returns container name for developer
def runUrlGuard(developer: str = "demo", routeId: str = "NONE", url: str = "/?path=/none"):
    if url.__contains__("force_to_allow=false"):
        print(f"<html><body><b>url:</b> {developer},{routeId},{url}, run {routeId.lower()}.sh</body></html>", end="")
        exit(9)
    print(f"<html><body>url fails: {developer},{routeId},{url}, run {routeId.lower()}.sh</body></html>", end="")

# this function returns Port number for RouteNameID
def getPortForRouteID(developer: str = "demo", portRouteNameId: str = "NONE"):
    out = subprocess.run(
        portForRouteID(developer, portRouteNameId), capture_output=True
    )
    result = out.stdout.decode().strip()
    print(result, end="")

# this function returns Port number for RouteNameID
def poster(developer: str = "demo", message: str = "NONE", containers: str = "NONE"):
    print(f"Poster => developer: {developer}, message: {message}, containers: {containers}", end="")
