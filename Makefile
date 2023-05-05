service:
	podman run --env-file=.env -p 8080:8080 -d --privileged -v szalaiti:/home/podman/ss -v sharecontainers:/home/podman/.local/share/containers p1p
run:
	podman run --env-file=.env -p 8080:8080 -it --privileged -v sharecontainers:/home/podman/.local/share/containers p1p bash
build-amd64:
	podman build --build-arg ARCH=amd64 --tag p1p .
build-arm64:
	podman build --build-arg ARCH=arm64 --tag p1p .

