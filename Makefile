test-integration:
	docker-compose -f docker-compose.test.yml up -d
	sleep 8
	migrate -path migrations -database "postgres://postgres:postgres@localhost:5433/digital_wallet_test?sslmode=disable&x-multi-statement=true" up
	go test -tags=integration -race ./test/integration/... -v
	docker-compose -f docker-compose.test.yml down

migrate-up:
	migrate -path migrations -database "postgres://postgres:postgres@localhost:5432/digital_wallet_api?sslmode=disable&x-multi-statement=true" up

migrate-down:
	migrate -path migrations -database "postgres://postgres:postgres@localhost:5432/digital_wallet_api?sslmode=disable&x-multi-statement=true" down

wire:
	cd cmd/api && wire

mock:
	go generate ./...

test-unit:
	go clean -testcache && go test ./... -v -race -cover

swag:
	swag init -g cmd/api/main.go