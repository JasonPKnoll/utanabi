# Thin wrapper over the Taskfile. Tasks run through the wails3 CLI pinned in go.mod, so no other install is needed.
TASK := go tool wails3 task

.PHONY: tools dev build run test test-integration test-e2e lint format vulncheck tidy bindings

# Installs wails3 on your PATH (GOBIN), which `make dev` and the build tasks call directly.
tools:
	go install tool

dev:
	$(TASK) dev

build:
	$(TASK) build

run:
	$(TASK) run

test:
	$(TASK) test

test-integration:
	$(TASK) test-integration

test-e2e:
	$(TASK) test:e2e

lint:
	$(TASK) lint

format:
	$(TASK) format

vulncheck:
	$(TASK) vulncheck

tidy:
	$(TASK) tidy

bindings:
	$(TASK) bindings
