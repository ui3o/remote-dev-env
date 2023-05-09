service:
	podman run --env-file=.env -p 8080:8080 -d --privileged -v sharecontainers:/home/podman/.local/share/containers local-remote-dev-env
run:
	podman run --env-file=.env -p 8080:8080 -it --privileged -v sharecontainers:/home/podman/.local/share/containers local-remote-dev-env bash
build-amd64:
	podman build --build-arg=ARCH=amd64 --tag local-remote-dev-env .
build-arm64:
	podman build --build-arg=ARCH=arm64 --tag local-remote-dev-env .
