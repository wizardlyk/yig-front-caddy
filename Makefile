.PHONY: build

build:
    go get -u github.com/caddyserver/builds
	cd caddy && go run build.go



