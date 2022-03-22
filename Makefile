# SPDX-FileCopyrightText: 2019-present Open Networking Foundation <info@opennetworking.org>
#
# SPDX-License-Identifier: Apache-2.0

export CGO_ENABLED=1
export GO111MODULE=on

.PHONY: build

ONOS_TOPO_VERSION := latest
ONOS_PROTOC_VERSION := v0.6.3

build-tools:=$(shell if [ ! -d "./build/build-tools" ]; then cd build && git clone https://github.com/onosproject/build-tools.git; fi)
include ./build/build-tools/make/onf-common.mk

mod-update: # @HELP Download the dependencies to the vendor folder
	go mod tidy
	go mod vendor
mod-lint: mod-update # @HELP ensure that the required dependencies are in place
	# dependencies are vendored, but not committed, go.sum is the only thing we need to check
	bash -c "diff -u <(echo -n) <(git diff go.sum)"

build: # @HELP build the Go binaries and run all validations (default)
build:
	CGO_ENABLED=1 go build -o build/_output/onos-uenib ./cmd/onos-uenib

test: # @HELP run the unit tests and source code validation producing a golang style report
test: mod-lint build linters license
	go test -race github.com/onosproject/onos-uenib/...

jenkins-test: # @HELP run the unit tests and source code validation producing a junit style report for Jenkins
jenkins-test: mod-lint build linters license
	TEST_PACKAGES=github.com/onosproject/onos-uenib/pkg/... ./build/build-tools/build/jenkins/make-unit

helmit-uenib: integration-test-namespace # @HELP run helmit tests locally
	helmit test -n test ./cmd/onos-uenib-tests --suite uenib

integration-tests: helmit-uenib # @HELP run helmit integration tests locally

onos-uenib-docker: # @HELP build onos-uenib base Docker image
	@go mod vendor
	docker build . -f build/onos-uenib/Dockerfile \
		-t onosproject/onos-uenib:${ONOS_TOPO_VERSION}
	@rm -rf vendor

images: # @HELP build all Docker images
images: build onos-uenib-docker

kind: # @HELP build Docker images and add them to the currently configured kind cluster
kind: images
	@if [ "`kind get clusters`" = '' ]; then echo "no kind cluster found" && exit 1; fi
	kind load docker-image onosproject/onos-uenib:${ONOS_TOPO_VERSION}

all: build images

publish: # @HELP publish version on github and dockerhub
	./build/build-tools/publish-version ${VERSION} onosproject/onos-uenib

jenkins-publish: jenkins-tools # @HELP Jenkins calls this to publish artifacts
	./build/bin/push-images
	./build/build-tools/release-merge-commit
	./build/build-tools/build/docs/push-docs

clean:: # @HELP remove all the build artifacts
	rm -rf ./build/_output ./vendor ./cmd/onos-uenib/onos-uenib ./cmd/dummy/dummy

