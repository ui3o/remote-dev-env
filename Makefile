service:
	podman run --env-file=.env -p 8080:8080 -d --privileged -v sharecontainers:/home/podman/.local/share/containers local-remote-dev-env
run:
	podman run --env-file=.env -v /etc/timezone:/etc/timezone -p 9568:9568 -p 8080:8080 -v sharecontainers:/home/podman/.local/share/containers  -it --privileged  localhost/local-remote-dev-env:latest
wget-service-types:
	rm -f types.d.ts
	wget https://raw.githubusercontent.com/ui3o/process-list-manager/main/types.d.ts 
build-amd64: wget-service-types
	podman build --build-arg=ARCH=amd64 --tag local-remote-dev-env .
build-arm64:
	podman build --build-arg=ARCH=arm64 --tag local-remote-dev-env .
