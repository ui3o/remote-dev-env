run:
	GOARCH=arm64 GOOS=linux go build main.go
	podman build -t gochildertest .
	podman run -it --rm --name gochildertest gochildertest
test:
	podman exec -it gochildertest sh
