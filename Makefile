.PHONY: up down logs up-auth down-auth logs-auth up-all down-all logs-all

# a) Caddy only
up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

# b) Caddy + Auth
up-auth:
	docker compose -f docker-compose.yml -f docker-compose.auth.yml up -d

down-auth:
	docker compose -f docker-compose.yml -f docker-compose.auth.yml down

logs-auth:
	docker compose -f docker-compose.yml -f docker-compose.auth.yml logs -f

# c) All (add more -f flags as needed)
up-all:
	docker compose -f docker-compose.yml -f docker-compose.auth.yml up -d

down-all:
	docker compose -f docker-compose.yml -f docker-compose.auth.yml down

logs-all:
	docker compose -f docker-compose.yml -f docker-compose.auth.yml logs -f
