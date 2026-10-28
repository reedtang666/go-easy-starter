GO ?= go
GF ?= .tools/gf
CONFIG ?= manifest/config/config.yaml
MIGRATIONS ?= manifest/sql

.PHONY: setup tools deps dev http queue scheduler build test test-integration race vet check ctrl service dao module db-new db-status db-up db-baseline db-init image
setup:
	$(GO) run . setup --config "$(CONFIG)"
tools:
	sh scripts/install-tools.sh
deps:
	docker compose up -d postgres redis
dev:
	$(GF) run main.go --args "all --config $(CONFIG)"
http:
	$(GO) run . http --config "$(CONFIG)"
queue:
	$(GO) run . queue --config "$(CONFIG)"
scheduler:
	$(GO) run . scheduler --config "$(CONFIG)"
build:
	mkdir -p bin
	$(GO) build -trimpath -o bin/starter .
test:
	$(GO) test ./...
test-integration:
	$(GO) test -tags=integration ./internal/bootstrap ./internal/migration ./internal/scaffold -count=1 -timeout=180s
race:
	$(GO) test -race ./...
vet:
	$(GO) vet ./...
check: test vet build
ctrl:
	$(GF) gen ctrl
service:
	$(GF) gen service
dao:
	$(GF) gen dao
db-init:
	$(GO) run . db-init --config "$(CONFIG)" --dir "$(MIGRATIONS)"
module:
	$(GO) run . module --name "$(NAME)"
db-new:
	$(GO) run . db-new --name "$(NAME)" --dir "$(MIGRATIONS)"
db-status:
	$(GO) run . db-status --config "$(CONFIG)" --dir "$(MIGRATIONS)"
db-up:
	$(GO) run . db-up --config "$(CONFIG)" --dir "$(MIGRATIONS)"
db-baseline:
	$(GO) run . db-baseline --config "$(CONFIG)" --dir "$(MIGRATIONS)"
image:
	docker build -f manifest/docker/Dockerfile -t go-easy-starter:local .
