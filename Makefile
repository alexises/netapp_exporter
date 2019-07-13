

GO           ?= go
GOFMT        ?= $(GO)fmt
FIRST_GOPATH := $(firstword $(subst :, ,$(shell $(GO) env GOPATH)))
STATICCHECK  := $(FIRST_GOPATH)/bin/staticcheck
GOVENDOR     := $(FIRST_GOPATH)/bin/govendor
GODEP				 := $(FIRST_GOPATH)/bin/dep
RPM          := ./scripts/build_rpm.sh
pkgs          = ./...

BIN_DIR                 ?= $(shell pwd)/build
VERSION ?= $(shell cat VERSION)
REVERSION ?=$(shell git log -1 --pretty="%H")
BRANCH ?=$(shell git rev-parse --abbrev-ref HEAD)
TIME ?=$(shell date)
HOST ?=$(shell hostname)  

all:  vet fmt style staticcheck unused  build buildrpm test

 
style:
	@echo ">> checking code style"
	! $(GOFMT) -d $$(find . -path ./vendor -prune -o -name '*.go' -print) | grep '^'

check_license:
	@echo ">> checking license header"
	@licRes=$$(for file in $$(find . -type f -iname '*.go' ! -path './vendor/*') ; do \
               awk 'NR<=3' $$file | grep -Eq "(Copyright|generated|GENERATED)" || echo $$file; \
       done); \
       if [ -n "$${licRes}" ]; then \
               echo "license header checking failed:"; echo "$${licRes}"; \
               exit 1; \
       fi

test-short:
	@echo ">> running short tests"
	$(GO) test -short $(pkgs)

test:
	@echo ">> running all tests"
	$(GO) test -race $(pkgs)

format:
	@echo ">> formatting code"
	$(GO) fmt $(pkgs)

vet:
	@echo ">> vetting code"
	$(GO) vet $(pkgs)

staticcheck: | $(STATICCHECK)
	@echo ">> running staticcheck"
	$(STATICCHECK) -ignore "$(STATICCHECK_IGNORE)" $(pkgs)

unused: 
	@echo ">> running check for unused packages"
	@$(GOVENDOR) list +unused | grep . && exit 1 || echo 'No unused packages'

build: 
	@echo ">> building binaries"
	$(GO) build  -o $(BIN_DIR)/netapp_exporter  -ldflags  '-X "main.Vsersion=$(VERSION)" -X  "main.BuildRevision=$(REVERSION)" -X  "main.BuildBranch=$(BRANCH)" -X "main.BuildTime=$(TIME)" -X "main.BuildHost=$(HOST)"'

buildrpm: | build
	@echo ">> building binaries"
	$(RPM) build

fmt:
	@echo ">> format code style"
	$(GOFMT) -w $$(find . -path ./vendor -prune -o -name '*.go' -print) 



$(STATICCHECK):
	GOOS= GOARCH= $(GO) get -u honnef.co/go/tools/cmd/staticcheck

$(GOVENDOR):
	GOOS= GOARCH= $(GO) get -u github.com/kardianos/govendor

.PHONY: all style check_license format build test vet assets tarball fmt  $(GODEP)  $(PROMU) $(STATICCHECK) $(GOVENDOR) package