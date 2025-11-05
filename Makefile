## test: runs all tests
test:
	@go test -v ./...

## cover: opens coverage in browser
cover:
	@go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out

## coverage: displays test coverage
coverage:
	@go test -cover ./...

## build_cli: builds the command line tool rkt and copies it to myapp
build_cli:
	@go build -o ../app/rkt ./cmd/cli

## build: builds the command line tool dist directory
build:
	@go build -o ./dist/rkt.exe ./cmd/cli

install_cli:
	@go build -o ~/go/bin/rkt -ldflags '-s -w' ./cmd/cli