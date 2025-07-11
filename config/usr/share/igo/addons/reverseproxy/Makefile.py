import logging
import os
import subprocess
import gen_user_lock
import time

PODMAN_REMOTE = "-v r_dev_shared_runtime:/tmp/.runtime \
        -v /run/user/1000/podman/podman.sock:/run/podman/podman.sock:ro \
        -v r_dev_shared_vol:/var/lib/shared-containers \
        -e DOCKER_HOST=unix:///run/podman/podman.sock \
        localhost/local-remote-dev-env:latest"
DEV_CONT_REMOTE_OPTS = os.getenv("DEV_CONT_REMOTE_OPTS", PODMAN_REMOTE)

HOME_FOLDER_PATH = os.getenv("ENV_PARAM_REVERSEPROXY_HOME_FOLDER_PATH", "")


def podmanStart(developer="demo", portLock: int = 9000):
    # todo list all portRSH, portCODE
    p = f"\
        podman --remote run -d --rm --privileged --name rdev-{developer} --network host\
            --label portLock={portLock}\
            -e DEVELOPER={developer}\
            -e DEV_CONT_MODE_NO_REVERSEPROXY=true\
            --mount=type=bind,source=/etc/localtime,target=/etc/localtime,ro\
            -v {HOME_FOLDER_PATH}{developer}:/root/ss:Z \
            {gen_user_lock.createLabelList(developer, portLock)}\
            {DEV_CONT_REMOTE_OPTS}\
        ".split(" ")
    return [arg for arg in p if arg]


def podmanCheckRun(developer="demo"):
    p = f"podman --remote container --filter=name=rdev-{developer} --format {{.Names}}".split(
        " "
    )
    return [arg for arg in p if arg]


def portLocksList():
    p = [
        *"\
        podman --remote ps --filter=name=rdev-.* --format {{.Labels.portLock}}\
        ".split(" ")
    ]
    return [arg for arg in p if arg]


def runningContainerList():
    p = [
        *"\
        podman --remote ps --filter=name=rdev-.* --format {{.Labels.DEVELOPEREnv}}\
        ".split(" ")
    ]
    return [arg for arg in p if arg]


def portForRouteID(developer="demo", portRouteNameId: str = "NONE"):
    p = [
        *(
            "\
        podman --remote ps --filter=name=rdev-"
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
    p = [*(f"podman --remote logs -f rdev-{developer}").split(" ")]
    return [arg for arg in p if arg]


def calculateLockNum() -> int:
    out = subprocess.run(portLocksList(), capture_output=True)
    result = out.stdout.decode().split("\n")
    result = [arg for arg in result if arg]
    r = list(range(11100, 14000 + 1, 100))
    # remove item from r where result is the same
    r = [port for port in r if str(port) not in result]
    print(r)
    if len(r):
        return r[0]
    return 0


# this is a start function
def start(developer: str = "demo"):
    # todo set lock and all ports
    portLock: int = calculateLockNum()
    if portLock:
        logging.info(podmanStart(developer, portLock))
        subprocess.run(podmanStart(developer, portLock))

# this is a start function
def removeIdleUsers(idleTime: int = 1):
    for user in runningContainerList():
        user = user.strip()
        access_file = f"/tmp/.runtime/logins/{user}/.access"
        try:
            stat = os.stat(access_file)
            last_access = stat.st_mtime
            idle_seconds = idleTime * 60
            if time.time() - last_access > idle_seconds:
                # remove the container if idle
                subprocess.run(["podman", "--remote", "kill", f"rdev-{user}"])
        except FileNotFoundError:
            pass

# this function checks if the container is running and exit if not
def listenContainerRunning(developer: str = "demo"):
    logging.info(podmanWatchLogs(developer))
    subprocess.run(podmanWatchLogs(developer))


# this function returns Port number for RouteNameID
def getPortForRouteID(developer: str = "demo", portRouteNameId: str = "NONE"):
    out = subprocess.run(
        portForRouteID(developer, portRouteNameId), capture_output=True
    )
    result = out.stdout.decode().strip()
    print(result, end="")

