.PHONY: build api clean dev-requirements test deubg dep-graph
VERSION := $(shell git describe --tags --always)

build: clean
	mkdir -p build
	go build $(GO_EXTRA_BUILD_ARGS) -ldflags "-s -w -X main.version=$(VERSION)" -o build/service cmd/main.go

api:
	buf generate
	@echo "Generating combined Swagger JSON"
	@GOOS="" GOARCH="" go run proto/main.go swagger/v1 > internal/server/statics/swagger/api.swagger.json
	@cp swagger/v1/*.json internal/server/statics/swagger

clean:
	rm -rf build
	rm -f internal/server/statics/*_gen.go

test:
	go test -race -cover -coverprofile coverage.out -coverpkg ./internal/... ./...
	# IMPORTANT: the coverage required can only be increased
	go tool cover -func coverage.out | \
		awk 'END { print "Coverage: " $$3; if ($$3 < 1) { print "Insufficient coverage"; exit 1; } }'

dev-requirements:
    # required if service implements GRPC APIs
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest
	go install github.com/go-bindata/go-bindata/go-bindata@latest

debug:
	## go test -c -gcflags="all=-N -l" ; dlv exec ./FILE.test
	go get -d github.com/go-delve/delve/cmd/dlv@v1.9.1
	go install github.com/go-delve/delve/cmd/dlv@v1.9.1

dep-graph:
	# first install tools:
	# sudo apt install graphviz
	# go install github.com/loov/goda
	goda graph -short 'github.com/niuroulamian/restful-api-service/...:root' | dot -Tpdf -o dep-graph.pdf

