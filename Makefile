-include .env
export $(shell sed 's/=.*//' .env)

lint:
	golangci-lint run

test:
	go test ./...

mempool:
	cd cmd/mempool && go run . -c ../../build/dipdup.yml

local:
	docker-compose -f docker-compose.local.yml up -d --build