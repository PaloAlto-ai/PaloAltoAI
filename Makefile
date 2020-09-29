# This Makefile is meant to be used by people that do not usually work
# with Go source code. If you know what GOPATH is then you probably
# don't need to bother with make.

.PHONY: gpaa android ios gpaa-cross swarm evm all test clean
.PHONY: gpaa-linux gpaa-linux-386 gpaa-linux-amd64 gpaa-linux-mips64 gpaa-linux-mips64le
.PHONY: gpaa-linux-arm gpaa-linux-arm-5 gpaa-linux-arm-6 gpaa-linux-arm-7 gpaa-linux-arm64
.PHONY: gpaa-darwin gpaa-darwin-386 gpaa-darwin-amd64
.PHONY: gpaa-windows gpaa-windows-386 gpaa-windows-amd64

GOBIN = $(shell pwd)/build/bin
GO ?= latest

gpaa:
	build/env.sh go run build/ci.go install ./cmd/gpaa
	@echo "Done building."
	@echo "Run \"$(GOBIN)/gpaa\" to launch gpaa."

swarm:
	build/env.sh go run build/ci.go install ./cmd/swarm
	@echo "Done building."
	@echo "Run \"$(GOBIN)/swarm\" to launch swarm."

all:
	build/env.sh go run build/ci.go install

android:
	build/env.sh go run build/ci.go aar --local
	@echo "Done building."
	@echo "Import \"$(GOBIN)/gpaa.aar\" to use the library."

ios:
	build/env.sh go run build/ci.go xcode --local
	@echo "Done building."
	@echo "Import \"$(GOBIN)/Gpaa.framework\" to use the library."

test: all
	build/env.sh go run build/ci.go test

lint: ## Run linters.
	build/env.sh go run build/ci.go lint

clean:
	./build/clean_go_build_cache.sh
	rm -fr build/_workspace/pkg/ $(GOBIN)/*

# The devtools target installs tools required for 'go generate'.
# You need to put $GOBIN (or $GOPATH/bin) in your PATH to use 'go generate'.

devtools:
	env GOBIN= go get -u golang.org/x/tools/cmd/stringer
	env GOBIN= go get -u github.com/kevinburke/go-bindata/go-bindata
	env GOBIN= go get -u github.com/fjl/gencodec
	env GOBIN= go get -u github.com/golang/protobuf/protoc-gen-go
	env GOBIN= go install ./cmd/abigen
	@type "npm" 2> /dev/null || echo 'Please install node.js and npm'
	@type "solc" 2> /dev/null || echo 'Please install solc'
	@type "protoc" 2> /dev/null || echo 'Please install protoc'

swarm-devtools:
	env GOBIN= go install ./cmd/swarm/mimegen

# Cross Compilation Targets (xgo)

gpaa-cross: gpaa-linux gpaa-darwin gpaa-windows gpaa-android gpaa-ios
	@echo "Full cross compilation done:"
	@ls -ld $(GOBIN)/gpaa-*

gpaa-linux: gpaa-linux-386 gpaa-linux-amd64 gpaa-linux-arm gpaa-linux-mips64 gpaa-linux-mips64le
	@echo "Linux cross compilation done:"
	@ls -ld $(GOBIN)/gpaa-linux-*

gpaa-linux-386:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/386 -v ./cmd/gpaa
	@echo "Linux 386 cross compilation done:"
	@ls -ld $(GOBIN)/gpaa-linux-* | grep 386

gpaa-linux-amd64:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/amd64 -v ./cmd/gpaa
	@echo "Linux amd64 cross compilation done:"
	@ls -ld $(GOBIN)/gpaa-linux-* | grep amd64

gpaa-linux-arm: gpaa-linux-arm-5 gpaa-linux-arm-6 gpaa-linux-arm-7 gpaa-linux-arm64
	@echo "Linux ARM cross compilation done:"
	@ls -ld $(GOBIN)/gpaa-linux-* | grep arm

gpaa-linux-arm-5:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/arm-5 -v ./cmd/gpaa
	@echo "Linux ARMv5 cross compilation done:"
	@ls -ld $(GOBIN)/gpaa-linux-* | grep arm-5

gpaa-linux-arm-6:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/arm-6 -v ./cmd/gpaa
	@echo "Linux ARMv6 cross compilation done:"
	@ls -ld $(GOBIN)/gpaa-linux-* | grep arm-6

gpaa-linux-arm-7:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/arm-7 -v ./cmd/gpaa
	@echo "Linux ARMv7 cross compilation done:"
	@ls -ld $(GOBIN)/gpaa-linux-* | grep arm-7

gpaa-linux-arm64:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/arm64 -v ./cmd/gpaa
	@echo "Linux ARM64 cross compilation done:"
	@ls -ld $(GOBIN)/gpaa-linux-* | grep arm64

gpaa-linux-mips:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/mips --ldflags '-extldflags "-static"' -v ./cmd/gpaa
	@echo "Linux MIPS cross compilation done:"
	@ls -ld $(GOBIN)/gpaa-linux-* | grep mips

gpaa-linux-mipsle:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/mipsle --ldflags '-extldflags "-static"' -v ./cmd/gpaa
	@echo "Linux MIPSle cross compilation done:"
	@ls -ld $(GOBIN)/gpaa-linux-* | grep mipsle

gpaa-linux-mips64:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/mips64 --ldflags '-extldflags "-static"' -v ./cmd/gpaa
	@echo "Linux MIPS64 cross compilation done:"
	@ls -ld $(GOBIN)/gpaa-linux-* | grep mips64

gpaa-linux-mips64le:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/mips64le --ldflags '-extldflags "-static"' -v ./cmd/gpaa
	@echo "Linux MIPS64le cross compilation done:"
	@ls -ld $(GOBIN)/gpaa-linux-* | grep mips64le

gpaa-darwin: gpaa-darwin-386 gpaa-darwin-amd64
	@echo "Darwin cross compilation done:"
	@ls -ld $(GOBIN)/gpaa-darwin-*

gpaa-darwin-386:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=darwin/386 -v ./cmd/gpaa
	@echo "Darwin 386 cross compilation done:"
	@ls -ld $(GOBIN)/gpaa-darwin-* | grep 386

gpaa-darwin-amd64:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=darwin/amd64 -v ./cmd/gpaa
	@echo "Darwin amd64 cross compilation done:"
	@ls -ld $(GOBIN)/gpaa-darwin-* | grep amd64

gpaa-windows: gpaa-windows-386 gpaa-windows-amd64
	@echo "Windows cross compilation done:"
	@ls -ld $(GOBIN)/gpaa-windows-*

gpaa-windows-386:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=windows/386 -v ./cmd/gpaa
	@echo "Windows 386 cross compilation done:"
	@ls -ld $(GOBIN)/gpaa-windows-* | grep 386

gpaa-windows-amd64:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=windows/amd64 -v ./cmd/gpaa
	@echo "Windows amd64 cross compilation done:"
	@ls -ld $(GOBIN)/gpaa-windows-* | grep amd64
