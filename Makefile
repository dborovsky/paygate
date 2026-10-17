DB_URL ?= postgres://paygate:paygate@localhost:5432/paygate?sslmode=disable

migrate-up:
	goose -dir migrations postgres "$(DB_URL)" up

migrate-down:
	goose -dir migrations postgres "$(DB_URL)" down

migrate-status:
	goose -dir migrations postgres "$(DB_URL)" status

check:
	go vet ./...
	go build ./...

