PLATFORM=$(shell podman info --format '{{.Host.Arch}}')
UID=$(shell id -u)

# on linux you can mount podman socket
# # -v /run/user/1000/podman/podman.sock:/run/podman/podman.sock:ro -e DOCKER_HOST=unix:///run/podman/podman.sock
# on MACOS you can mount podman socket
# # -e DOCKER_HOST=ssh://core@host.containers.intenal:50533/run/user/501/podman/podman.sock

PODMAN_REMOTE=-v /run/user/$(UID)/podman/podman.sock:/run/podman/podman.sock:ro \
		-e DOCKER_HOST=unix:///run/podman/podman.sock \
		-v r_dev_shared_vol:/var/lib/shared-containers \
		-v r_dev_shared_runtime:/tmp/.runtime \
		localhost/local-codebox:latest

# -v /tmp/.runtime:/tmp/.runtime \
# run target for Local Remote Dev Environment
run:
	/opt/homebrew/bin/podman run -it --rm --privileged --name codebox-admin-revproxy --network host \
		-e DEVELOPER=reverse_proxy \
		-e "CODEBOX_REMOTE_OPTS=$(PODMAN_REMOTE)" \
		-e "CODEBOX_ENABLED_ADDONS_LIST=reverseproxy" \
		-e "CODEBOX_MODE_DISABLE_UNITS=true" \
		-e "ENV_PARAM_REVERSEPROXY_LOCAL_GLOBAL_PORT_LIST=ADMIN;CODE;RSH;LOCAL1;LOCAL2|GRAFANA;GLOBAL1;GLOBAL2" \
		-e ENV_PARAM_REVERSEPROXY_REPLACE_SUBDOMAIN_TO_COOKIE=true \
		-e ENV_PARAM_REVERSEPROXY_PORT=10111 \
		--mount=type=bind,source=/etc/localtime,target=/etc/localtime,ro \
		$(PODMAN_REMOTE)

# build target for Local Remote Dev Environment
build:
	echo "Building for platform: $(PLATFORM)"
	podman build --platform=linux/$(PLATFORM) --build-arg=ARCH=$(PLATFORM) --tag local-codebox .
# make-cert target to generate a self-signed certificate
make-cert:
	rm -rf cert && mkdir cert
	openssl req -x509 -newkey rsa:4096 -keyout cert/server_key.pem -out cert/server_cert.pem -nodes -days 1825 -subj "/CN=*.localhost.com/O=root\ codebox\ certificate" -addext "subjectAltName=DNS:*.localhost.com"
