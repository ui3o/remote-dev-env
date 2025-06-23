PLATFORM=$(shell podman info --format '{{.Host.Arch}}')
UID=$(shell id -u)

# on linux you can mount podman socket
# # -v /run/user/1000/podman/podman.sock:/run/podman/podman.sock:ro -e DOCKER_HOST=unix:///run/podman/podman.sock
# on MACOS you can mount podman socket
# # -e DOCKER_HOST=ssh://core@host.containers.intenal:50533/run/user/501/podman/podman.sock

PODMAN_REMOTE=-v /run/user/$(UID)/podman/podman.sock:/run/podman/podman.sock:ro -e DOCKER_HOST=unix:///run/podman/podman.sock
ifeq ($(PLATFORM),arm64)
PODMAN_REMOTE=-e DOCKER_HOST=ssh://core@host.containers.intenal:50533/run/user/$(UID)/podman/podman.sock
endif

# run target for Local Remote Dev Environment
run:
	podman run --rm -p 10111:10111 -p 7681:7681 -p 8080:8080 \
		-p 11000:11000 -p 11001:11001 \
		-p 11100:11100 -p 11101:11101 \
		-e USERNAME=foo \
		-e DEV_CONT_MODE_NO_REVERSEPROXY=true \
		-e ENV_PARAM_REVERSEPROXY_PORT=10111 \
		--name rdev \
		--mount=type=bind,source=$(PWD)/tmp/timezone,target=/etc/timezone \
		-v sharedvol1:/var/lib/shared-containers \
		-v sharedtmplogins:/tmp/.logins \
		$(PODMAN_REMOTE) \
		-it --privileged \
		localhost/local-remote-dev-env:latest

# build target for Local Remote Dev Environment
build:
	echo "Building for platform: $(PLATFORM)"
	podman build --platform=linux/$(PLATFORM) --build-arg=ARCH=$(PLATFORM) --tag local-remote-dev-env .
# make-cert target to generate a self-signed certificate
make-cert:
	rm -rf cert && mkdir cert
	openssl req -x509 -newkey rsa:4096 -keyout cert/server_key.pem -out cert/server_cert.pem -nodes -days 1825 -subj "/CN=*.localhost.com/O=root\ remote-dev-env\ certificate" -addext "subjectAltName=DNS:*.localhost.com"