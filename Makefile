.PHONY: up down build test logs ps

up:
	docker compose up --build

down:
	docker compose down

build:
	go build ./...

test:
	go test ./...

logs:
	docker compose logs -f app postgres

ps:
	docker compose ps
