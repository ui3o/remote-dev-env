service:
	podman run --env-file=.env -p 8080:8080 -d --privileged -v szalaiti:/home/podman/ss -v sharecontainers:/home/podman/.local/share/containers local-remote-dev-env
run:
	podman run --env-file=.env -p 8080:8080 -it --privileged -v sharecontainers:/home/podman/.local/share/containers local-remote-dev-env bash
build:
	podman build --tag local-remote-dev-env .
