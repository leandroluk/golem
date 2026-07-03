GOLEM_TEST_DSN ?= postgres://golem:golem@localhost:55432/golem_test?sslmode=disable

.PHONY: build vet test test-integration gate-quick gate-full

build:
	go build ./...

vet:
	go vet ./...

test:
	go test ./... -short

test-integration:
	docker compose -f docker-compose.test.yml up -d --wait
	GOLEM_TEST_DSN=$(GOLEM_TEST_DSN) go test -tags=integration ./... ; status=$$? ; docker compose -f docker-compose.test.yml down ; exit $$status

gate-quick: build vet test

gate-full: gate-quick test-integration
