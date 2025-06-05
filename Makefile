run:
	podman run --rm -p 10111:10111 -p 7681:7681 -p 8080:8080 \
		-p 11000:11000 -p 11001:11001 \
		-p 11100:11100 -p 11101:11101 \
		--name rdev \
		--mount=type=bind,source=$(PWD)/tmp/timezone,target=/etc/timezone \
		-v sharedvol1:/var/lib/shared-containers \
		-it --privileged \
		localhost/local-remote-dev-env:latest
test-user-create:
	podman run -it --rm --privileged -v ./.config/etc/units:/etc/units -v ./.config/user:/home/podman fedorawithzsh
build-amd64:
	podman build --platform=linux/amd64 --build-arg=ARCH=amd64 --tag local-remote-dev-env .
build-arm64:
	podman build --platform=linux/arm64 --build-arg=ARCH=arm64 --tag local-remote-dev-env .
make-cert:
	rm -rf cert && mkdir cert
	openssl req -x509 -newkey rsa:4096 -keyout cert/server_key.pem -out cert/server_cert.pem -nodes -days 1825 -subj "/CN=*.localhost.com/O=root\ remote-dev-env\ certificate" -addext "subjectAltName=DNS:*.localhost.com"