FLAGS =
TESTENVVAR =
REGISTRY = 105716418883.dkr.ecr.us-west-2.amazonaws.com
TAG_PREFIX = v
VERSION = $(shell git describe --tags --always --dirty)
TAG = $(TAG_PREFIX)$(VERSION)
DOCKER_CLI ?= docker
PKGS = $(shell go list ./... | grep -v /vendor/ | grep -v /tests/e2e)
ARCH ?= $(shell go env GOARCH)
BuildDate = $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
Commit = $(shell git rev-parse --short HEAD)
ALL_ARCH = amd64 arm arm64 ppc64le s390x
PKG = k8s.io/kube-event-exporter/pkg
GO_VERSION = 1.14.1
FIRST_GOPATH := $(firstword $(subst :, ,$(shell go env GOPATH)))
BENCHCMP_BINARY := $(FIRST_GOPATH)/bin/benchcmp
GOLANGCI_VERSION := v1.25.0
HAS_GOLANGCI := $(shell which golangci-lint)

IMAGE = $(REGISTRY)/kube-event-exporter
MULTI_ARCH_IMG = $(IMAGE)-$(ARCH)

validate-modules:
	@echo "- Verifying that the dependencies have expected content..."
	go mod verify
	@echo "- Checking for any unused/missing packages in go.mod..."
	go mod tidy
	@echo "- Checking for unused packages in vendor..."
	go mod vendor
	@git diff --exit-code -- go.sum go.mod vendor/

lint: shellcheck licensecheck
ifndef HAS_GOLANGCI
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(GOPATH)/bin ${GOLANGCI_VERSION}
endif
	golangci-lint run

build-local: clean
	GOOS=$(shell uname -s | tr A-Z a-z) GOARCH=$(ARCH) CGO_ENABLED=0 go build -ldflags "-s -w -X ${PKG}/version.Release=${TAG} -X ${PKG}/version.Commit=${Commit} -X ${PKG}/version.BuildDate=${BuildDate}" -o kube-event-exporter

build: clean kube-event-exporter

kube-event-exporter:
	${DOCKER_CLI} run --rm -v "${PWD}:/go/src/k8s.io/kube-event-exporter" -w /go/src/k8s.io/kube-event-exporter golang:${GO_VERSION} make build-local

shellcheck:
	${DOCKER_CLI} run -v "${PWD}:/mnt" koalaman/shellcheck:stable $(shell find . -type f -name "*.sh" -not -path "*vendor*")

TEMP_DIR := $(shell mktemp -d)

container: .container-$(ARCH)
.container-$(ARCH): kube-event-exporter
	cp -r * "${TEMP_DIR}"
	${DOCKER_CLI} build -t $(MULTI_ARCH_IMG):$(TAG) "${TEMP_DIR}"
	${DOCKER_CLI} tag $(MULTI_ARCH_IMG):$(TAG) $(MULTI_ARCH_IMG):latest
	rm -rf "${TEMP_DIR}"

ifeq ($(ARCH), amd64)
	# Adding check for amd64
	${DOCKER_CLI} tag $(MULTI_ARCH_IMG):$(TAG) $(IMAGE):$(TAG)
	${DOCKER_CLI} tag $(MULTI_ARCH_IMG):$(TAG) $(IMAGE):latest
endif

clean:
	rm -f kube-event-exporter
	git clean -Xfd .

# .PHONY: all build build-local all-push all-container test-unit test-benchmark-compare container push quay-push clean e2e validate-modules shellcheck licensecheck lint generate embedmd
.PHONY: validate-modules lint shellcheck container clean build build-local
