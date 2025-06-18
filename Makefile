PLATFORM=$(shell podman info --format '{{.Host.Arch}}')
# run target for Local Remote Dev Environment
run:
	podman run --rm -p 10111:10111 -p 7681:7681 -p 8080:8080 \
		-p 11000:11000 -p 11001:11001 \
		-p 11100:11100 -p 11101:11101 \
		-e ENV_PARAM_REVERSEPROXY_PORT=10111 \
		--name rdev \
		--mount=type=bind,source=$(PWD)/tmp/timezone,target=/etc/timezone \
		-v sharedvol1:/var/lib/shared-containers \
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