run:
	podman run --rm -p 10111:10111 -p 7681:7681 -p 8080:8080 \
		--name rdev \
		--mount=type=bind,source=$(PWD)/tmp/timezone,target=/etc/timezone \
		-v sharedvol1:/var/lib/shared-containers \
		-it --privileged \
		localhost/local-remote-dev-env:latest
build-amd64: wget-service-types
	podman build --platform=linux/amd64 --build-arg=ARCH=amd64 --tag local-remote-dev-env .
build-arm64:
	podman build --platform=linux/arm64 --build-arg=ARCH=arm64 --tag local-remote-dev-env .
make-cert:
	rm -rf cert && mkdir cert
	openssl req -x509 -newkey rsa:4096 -keyout cert/server_key.pem -out cert/server_cert.pem -nodes -days 1825 -subj "/CN=ice.corp.us/O=root\ ice\ certificate"
	openssl req -newkey rsa:4096 -keyout cert/wildcard_ice_key.pem -out cert/wildcard_ice_csr.pem -nodes -days 1825 -subj "/CN=*.ice.corp.us/O=corp.us/OU=us/L=London/ST=London/C=us"
	openssl x509 -req -in cert/wildcard_ice_csr.pem -CA cert/server_cert.pem -CAkey cert/server_key.pem -out cert/wildcard_ice_cert.pem -days 1825 -extfile openssl.cnf
