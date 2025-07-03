import logging
import os
import subprocess

PODMAN_REMOTE = "-v r_dev_shared_runtime:/tmp/.runtime \
        -v /run/user/1000/podman/podman.sock:/run/podman/podman.sock:ro \
        -v r_dev_shared_vol:/var/lib/shared-containers \
        -e DOCKER_HOST=unix:///run/podman/podman.sock"
DEV_CONT_REMOTE_OPTS = os.getenv("DEV_CONT_REMOTE_OPTS", PODMAN_REMOTE)

def podman(developer="demo"):
    p = f"\
        podman --remote run -d --rm --privileged --name rdev-{developer} --network host\
            -e DEVELOPER={developer}\
            -e DEV_CONT_MODE_NO_REVERSEPROXY=true\
            --mount=type=bind,source=/etc/localtime,target=/etc/localtime,ro\
            {DEV_CONT_REMOTE_OPTS}\
            localhost/local-remote-dev-env:latest\
        ".split(" ")
    return [arg for arg in p if arg]


def secretMove(developer="demo"):
    p = [
        *f"\
        podman --remote exec -it rdev-{developer} bash -c \
        ".split(" "),
        f"mv /tmp/.runtime/logins/{developer}/localstorage /run/secrets/localstorage && touch /tmp/.runtime/logins/{developer}/localstorage.synced",
    ]
    return [arg for arg in p if arg]


# this is a start function
def start(developer: str = "demo"):
    logging.info(podman(developer))
    subprocess.run(podman(developer))


# move secret to /run/secret
def store_secret(developer: str = "demo"):
    logging.info(secretMove(developer))
    subprocess.run(secretMove(developer))
