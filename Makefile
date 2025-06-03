# demo for sg
service:
	podman run --env-file=.env -p 8080:8080 -d --privileged -v sharecontainers:/home/podman/.local/share/containers local-remote-dev-env
# -e CERTS="$(CERTS)"
run:
	podman run --rm -p 9876:7681 -p 8080:8080 \
		--name rdev \
		--mount=type=bind,source=$(PWD)/tmp/timezone,target=/etc/timezone \
		-v sharedvol1:/var/lib/shared-containers \
		-it --privileged \
		localhost/local-remote-dev-env:latest
wget-service-types:
	rm -f types.d.ts
	wget https://raw.githubusercontent.com/ui3o/process-list-manager/main/types.d.ts 
build-amd64: wget-service-types
	podman build --platform=linux/amd64 --build-arg=ARCH=amd64 --tag local-remote-dev-env .
build-arm64:
	podman build --platform=linux/arm64 --build-arg=ARCH=arm64 --tag local-remote-dev-env .
make-cert:
	rm -rf cert && mkdir cert
	openssl req -x509 -newkey rsa:4096 -keyout cert/server_key.pem -out cert/server_cert.pem -nodes -days 1825 -subj "/CN=ice.corp.us/O=root\ ice\ certificate"
	openssl req -newkey rsa:4096 -keyout cert/wildcard_ice_key.pem -out cert/wildcard_ice_csr.pem -nodes -days 1825 -subj "/CN=*.ice.corp.us/O=corp.us/OU=us/L=London/ST=London/C=us"
	openssl x509 -req -in cert/wildcard_ice_csr.pem -CA cert/server_cert.pem -CAkey cert/server_key.pem -out cert/wildcard_ice_cert.pem -days 1825 -extfile openssl.cnf
test-inits-arm64:
	podman build --build-arg=CERTS="$(CERTS)" --platform=linux/arm64 -f test/inisys/Dockerfile --tls-verify=false --tag local-inits .
	podman run --rm -v ${PWD}/.config/etc/inits:/etc/inits -it --name linits local-inits
test-nix-bundle-build:
	podman build --build-arg=CERTS="$(CERTS)" --platform=linux/arm64 -f test/nix-bundle/Dockerfile --tls-verify=false --tag local-nix-bundle .
test-nix-bundle-run:
	podman run --privileged -v $(PWD)/test/nix-bundle/:/app -e CERTS="$(CERTS)" --rm -it --name lnb local-nix-bundle
test-alpine-run:
	podman run --privileged -e CERTS="$(CERTS)" --rm -it --name lnb local-nix-bundle