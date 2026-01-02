.PHONY: up down logs up-auth down-auth logs-auth

# Main caddy stack
up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

# With auth (tinyauth)
up-auth:
	docker compose -f docker-compose.yml -f docker-compose.auth.yml up -d

down-auth:
	docker compose -f docker-compose.yml -f docker-compose.auth.yml down

logs-auth:
	docker compose -f docker-compose.yml -f docker-compose.auth.yml logs -f
