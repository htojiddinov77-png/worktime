SHELL := /bin/bash

GOPATH := $(shell go env GOPATH)
GOBIN := $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(GOPATH)/bin
endif
GOOSE := $(GOBIN)/goose

-include .env
export

.PHONY: run deps goose migrate check-env db

run: deps goose migrate
	go run .

deps:
	go mod download

goose:
	@if [ ! -x "$(GOOSE)" ]; then \
		echo "Installing goose..."; \
		go install github.com/pressly/goose/v3/cmd/goose@v3.26.0; \
	fi

check-env:
	@if [ -z "$(WORKTIME_DB_DSN)" ]; then \
		echo "WORKTIME_DB_DSN is not set."; \
		exit 1; \
	fi

migrate: check-env goose
	$(GOOSE) -dir migrations postgres "$(WORKTIME_DB_DSN)" up

db:
	createdb worktime
