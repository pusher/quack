include .env
BINARY := quack
VERSION := $(shell git describe --always --dirty --tags 2>/dev/null || echo "undefined")

RED := \033[31m
GREEN := \033[32m
NC := \033[0m

IMG ?= quay.io/pusher/quack

.NOTPARALLEL:

.PHONY: all
all: distclean test build

.PHONY: build
build: clean $(BINARY)

.PHONY: clean
clean:
	rm -f $(BINARY)

.PHONY: distclean
distclean: clean
	rm -rf vendor
	rm -rf release

.PHONY: fmt
fmt:
	$(GO) fmt ./cmd/... ./pkg/...

.PHONY: vet
vet: vendor
	$(GO) vet ./cmd/... ./pkg/...

.PHONY: lint
lint:
	@ echo -e "$(GREEN)Linting code$(NC)"
	$(LINTER) run --disable-all \
		--exclude-use-default=false \
		--enable=govet \
		--enable=ineffassign \
		--enable=deadcode \
		--enable=golint \
		--enable=goconst \
		--enable=gofmt \
		--enable=goimports \
		--deadline=120s \
		--tests ./...
	@ echo

vendor:
	@ echo -e "$(GREEN)Pulling dependencies$(NC)"
	$(DEP) ensure -v --vendor-only
	@ echo

.PHONY: test
test: vendor
	@ echo -e "$(GREEN)Running test suite$(NC)"
	$(GO) test ./...
	@ echo

.PHONY: check
check: fmt lint vet test

.PHONY: build
build: clean $(BINARY)

$(BINARY): fmt vet
	CGO_ENABLED=0 $(GO) build -o $(BINARY) github.com/pusher/quack/cmd/quack

.PHONY: docker-build
docker-build: check
	docker build . -t ${IMG}:${VERSION}
	@echo -e "$(GREEN)Built $(IMG):$(VERSION)$(NC)"

TAGS ?= latest
.PHONY: docker-tag
docker-tag: docker-build
	@IFS=","; tags=${TAGS}; for tag in $${tags}; do docker tag ${IMG}:${VERSION} ${IMG}:$${tag}; echo -e "$(GREEN)Tagged $(IMG):$(VERSION) as $${tag}$(NC)"; done

PUSH_TAGS ?= ${VERSION}, latest
.PHONY: docker-push
docker-push: docker-build docker-tag
	@IFS=","; tags=${PUSH_TAGS}; for tag in $${tags}; do docker push ${IMG}:$${tag}; echo -e "$(GREEN)Pushed $(IMG):$${tag}$(NC)"; done

TAGS ?= latest
.PHONY: docker-clean
docker-clean:
	@IFS=","; tags=${TAGS}; for tag in $${tags}; do docker rmi -f ${IMG}:${VERSION} ${IMG}:$${tag}; done
