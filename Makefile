########################################################################################################################
# Copyright (c) 2019 IoTeX
# This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
# warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
# permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
# License 2.0 that can be found in the LICENSE file.
########################################################################################################################

# Go parameters
GOCMD=go
GOLINT=golint
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
BUILD_TARGET_SERVER=server

# Pkgs
ALL_PKGS := $(shell go list ./... )
PKGS := $(shell go list ./... | grep -v /test/ )
ROOT_PKG := "github.com/iotexproject/iotex-analytics"

# Docker parameters
DOCKERCMD=docker

all: clean build test

.PHONY: build
build:
	$(GOBUILD) -o ./bin/$(BUILD_TARGET_SERVER) -v .

.PHONY: fmt
fmt:
	$(GOCMD) fmt ./...

.PHONY: lint
lint:
	go list ./... | grep -v /vendor/ | xargs $(GOLINT)

.PHONY: test
test: fmt
	$(GOTEST) -short -p 1 ./...

.PHONY: clean
clean:
	@echo "Cleaning..."
	$(ECHO_V)rm -rf ./bin/$(BUILD_TARGET_SERVER)
	$(ECHO_V)$(GOCLEAN) -i $(PKGS)

.PHONY: run
run:
	$(GOBUILD) -o ./bin/$(BUILD_TARGET_SERVER) -v .
	./bin/$(BUILD_TARGET_SERVER)

.PHONY: docker
docker:
	$(DOCKERCMD) build -t $(USER)/iotex-analytics:latest .
