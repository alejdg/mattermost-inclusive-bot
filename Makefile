GO ?= $(shell command -v go 2> /dev/null)

## Define the default target (make all)
.PHONY: default
default: all

.PHONY: build
build:
	env CGO_ENABLED=0 $(GO) build

image-build:
	docker build . -t inclusive-bot:dev

.PHONY: mattermost-up
mattermost-up:
	cd dev/; docker-compose up -d

.PHONY: mattermost-down
mattermost-down:
	cd dev/; docker-compose down

# needs fixing
.PHONY: bot-up
bot-up:
	docker run --name=inclusive-bot --network=dev_default --rm inclusive-bot

.PHONY: bot-down
bot-down:
	docker stop inclusive-bot

.PHONY: check-style
check-style:
	@if ! [ -x "$$(command -v golangci-lint)" ]; then \
		echo "golangci-lint is not installed. Please see https://github.com/golangci/golangci-lint#install for installation instructions."; \
		exit 1; \
	fi; \

	@echo Running golangci-lint
	cd app/; golangci-lint run ./...