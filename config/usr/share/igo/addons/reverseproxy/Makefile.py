import logging
import os
import subprocess
import sys

PODMAN_COMMAND = ["podman", "machine", "inspect", "--format", "{{.SSHConfig.Port}}"]
UID = os.getuid()
PODMAN_REMOTE = f"-v /run/user/{UID}/podman/podman.sock:/run/podman/podman.sock:ro \
        -v r_dev_shared_runtime:/tmp/.runtime\
        -e DOCKER_HOST=unix:///run/podman/podman.sock"

if sys.platform == "darwin":
    p = subprocess.run(PODMAN_COMMAND, capture_output=True).stdout.decode("utf-8")
    PODMAN_REMOTE = (
        f"-e DOCKER_HOST=ssh://core@host.containers.intenal:{p.strip()}/run/user/{UID}/podman/podman.sock\
        -v /tmp/.runtime:/tmp/.runtime"
    )


def podman(username="demo"):
    p = f"\
        podman --remote run -d --rm --name rdev-{username} --network host --privileged\
            -e USERNAME={username}\
            -e DEV_CONT_MODE_NO_REVERSEPROXY=true\
            --mount=type=bind,source=/etc/localtime,target=/etc/localtime,ro\
            -v r_dev_shared_vol:/var/lib/shared-containers\
            {PODMAN_REMOTE}\
            localhost/local-remote-dev-env:latest\
        ".split(" ")
    return [arg for arg in p if arg]


# this is a start function
def start(username: str = "demo"):
    logging.info(podman(username))
    subprocess.run(podman(username))
