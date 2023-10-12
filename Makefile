service:
	podman run --env-file=.env -p 8080:8080 -d --privileged -v sharecontainers:/home/podman/.local/share/containers local-remote-dev-env
run:
	podman run --env-file=.env -v /etc/timezone:/etc/timezone -p 9568:9568 -p 8080:8080 -v sharedvol1:/var/lib/shared-containers -it --privileged localhost/local-remote-dev-env:latest
wget-service-types:
	rm -f types.d.ts
	wget https://raw.githubusercontent.com/ui3o/process-list-manager/main/types.d.ts 
build-amd64: wget-service-types
	podman build --build-arg=ARCH=amd64 --tag local-remote-dev-env .
build-arm64:
	podman build --build-arg=ARCH=arm64 --tag local-remote-dev-env .
make-cert:
	openssl req -x509 -newkey rsa:4096 -keyout cert/server_key.pem -out cert/server_cert.pem -nodes -days 1825 -subj "/CN=ice.corp.otpbank.hu/O=root\ ice\ certificate"
	openssl req -newkey rsa:4096 -keyout cert/wildcard_ice_key.pem -out cert/wildcard_ice_csr.pem -nodes -days 1825 -subj "/CN=*.ice.corp.otpbank.hu/O=corp.otpbank.hu/OU=otpbank.hu/L=Budapest/ST=Budapest/C=HU"
	openssl x509 -req -in cert/wildcard_ice_csr.pem -CA cert/server_cert.pem -CAkey cert/server_key.pem -out cert/wildcard_ice_cert.pem -days 1825 -extfile openssl.cnf
