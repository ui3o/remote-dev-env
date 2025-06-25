import logging
import os
import subprocess
import sys

PWD = os.getcwd() + "/../../../../../.."
UID = os.getuid()
PODMAN_REMOTE = (
    f"-e DOCKER_HOST=ssh://core@host.containers.intenal:50533/run/user/{UID}/podman/podman.sock"
    if sys.platform == "darwin"
    else f"-v /run/user/{UID}/podman/podman.sock:/run/podman/podman.sock:ro -e DOCKER_HOST=unix:///run/podman/podman.sock"
)


def podman(username="demo"):
    p = f"\
        podman run --rm --name rdev -p 10111:10111 -p 7681:7681 -p 8080:8080\
            -p 11000:11000 -p 11001:11001\
           -p 11100:11100 -p 11101:11101\
            -e USERNAME={username}\
            -e DEV_CONT_MODE_NO_REVERSEPROXY=true\
            -e ENV_PARAM_REVERSEPROXY_PORT=10111\
            --mount=type=bind,source={PWD}/tmp/timezone,target=/etc/timezone\
            {PODMAN_REMOTE}\
            -v sharedvol1:/var/lib/shared-containers\
            -v sharedtmplogins:/tmp/.logins\
            -it --privileged\
            localhost/local-remote-dev-env:latest\
        ".split(" ")
    return [arg for arg in p if arg]


# this is a start function
def debug(mas: bool, username: float = 1.9, a: str = "sda"):
    logging.info(
        f"start podman image for {username}, with additional arg: {a} and mas: {mas}"
    )
    # logging.info(podman(username))


# this is a start function
def startedSta():
    logging.info("podman(username)")


# this is a start function
def startedStat(username: str = "a"):
    logging.info(podman(username))
    subprocess.run(podman(username))
