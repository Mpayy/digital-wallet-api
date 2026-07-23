test-integration:
	docker-compose -f docker-compose.test.yml up -d
	sleep 8
	migrate -path migrations -database "mysql://root@tcp(localhost:3307)/digital_wallet_test?multiStatements=true" up
	go test -tags=integration -race ./test/integration/... -v
	docker-compose -f docker-compose.test.yml down