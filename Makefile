ROOT_DIR := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
SHELL := /bin/sh
VERSION ?= $(shell git describe --tags --always --dirty --abbrev=0)

SOURCES = $(shell find $(ROOT_DIR) -name "*.go" -print | grep -v /vendor/)
TESTS = $(shell go list ./... | grep -v test/e2e)

GOOS ?= linux
GOARCH ?= amd64
GOPATH ?= $(shell pwd)
COVERAGE_DIR ?= $(PWD)/coverage

export GO111MODULE = on

default: all

all: build check

check: checkfmt test lint

vendor:
	go mod vendor
	go mod verify

clean:
	rm -f build/main

server: build
	go build  -o ./build/distrox ./cmd/distrox #-gcflags '-m -m'

run: fmt server
	./build/distrox

bench:
	GOMAXPROCS=4 go test ./pkg/distrox/ -bench='Set|Get' -benchtime=10s

test:
	go test -race -v $(TESTS)

coverage:
	mkdir -p $(COVERAGE_DIR)
	go test -v $(TESTS) -coverpkg=./... -coverprofile=$(COVERAGE_DIR)/coverage.out
	go test -v $(TESTS) -coverpkg=./... -covermode=count -coverprofile=$(COVERAGE_DIR)/count.out fmt
	go tool cover -func=$(COVERAGE_DIR)/coverage.out
	go tool cover -func=$(COVERAGE_DIR)/count.out
	go tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/index.html

checkfmt:
	@[ -z $$(gofmt -l $(SOURCES)) ] || (echo "Sources not formatted correctly. Fix by running: make fmt" && false)

fmt: $(SOURCES)
	gofmt -s -w $(SOURCES)
	goimports -w $(SOURCES)

lint:
	golangci-lint run
