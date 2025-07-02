import logging
import os
import subprocess

PODMAN_COMMAND = ["podman", "machine", "inspect", "--format", "{{.SSHConfig.Port}}"]
DEV_CONT_HOST_UID = os.getenv("DEV_CONT_HOST_UID", "1")
DEV_CONT_REMOTE_OPTS = os.getenv("DEV_CONT_REMOTE_OPTS", "")
UID = os.getuid()

PODMAN_REMOTE = f"-v r_dev_shared_runtime:/tmp/.runtime \
        -v /run/user/{DEV_CONT_HOST_UID}/podman/podman.sock:/run/podman/podman.sock:ro \
        -e DOCKER_HOST=unix:///run/podman/podman.sock"


def podman(developer="demo"):
    p = f"\
        podman --remote run -d --rm --name rdev-{developer} --network host --privileged\
            -e DEVELOPER={developer}\
            -e DEV_CONT_MODE_NO_REVERSEPROXY=true\
            --mount=type=bind,source=/etc/localtime,target=/etc/localtime,ro\
            -v r_dev_shared_vol:/var/lib/shared-containers\
            {PODMAN_REMOTE} {DEV_CONT_REMOTE_OPTS}\
            localhost/local-remote-dev-env:latest\
        ".split(" ")
    return [arg for arg in p if arg]


def secretMove(developer="demo"):
    p = [*f"\
        podman --remote exec -it rdev-{developer} bash -c \
        ".split(" "), f"mv /tmp/.runtime/logins/{developer}/localstorage /run/secrets/localstorage && touch /tmp/.runtime/logins/{developer}/localstorage.synced"]
    return [arg for arg in p if arg]


# this is a start function
def start(developer: str = "demo"):
    logging.info(podman(developer))
    subprocess.run(podman(developer))


# move secret to /run/secret
def store_secret(developer: str = "demo"):
    logging.info(secretMove(developer))
    subprocess.run(secretMove(developer))
