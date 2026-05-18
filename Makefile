.PHONY: build test test-threads test-processes test-burst

MEMORY_LIMIT ?= 512m
MEMORY_STEP  ?= 64
WORKERS      ?= 4

build:
	@mkdir -p bin
	go build -o bin/oom-signal ./src/cmd

test:
	go test -race ./src/...

test-threads:
	docker run --rm --memory=$(MEMORY_LIMIT) $(shell docker build -q -t oom-signal-test -f test/Dockerfile .) threads --step $(MEMORY_STEP) --workers $(WORKERS)

test-processes:
	docker run --rm --memory=$(MEMORY_LIMIT) $(shell docker build -q -t oom-signal-test -f test/Dockerfile .) processes --step $(MEMORY_STEP) --workers $(WORKERS)

test-burst:
	docker run --rm --memory=$(MEMORY_LIMIT) --entrypoint python $(shell docker build -q -t oom-signal-test -f test/Dockerfile .) /opt/oom-signal/burst_test.py
