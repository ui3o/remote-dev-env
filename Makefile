run:
	podman run --env-file=.env -p 8080:8080 -it --privileged -v sharecontainers:/home/podman/.local/share/containers p1p bash
build:
	podman build --tag p1p .
