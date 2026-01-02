.PHONY: up down logs up-auth down-auth logs-auth up-all down-all logs-all

# Compose files (always include all)
COMPOSE_FILES = -f docker-compose.yml -f docker-compose.auth.yml

# a) Caddy only
up:
	docker compose $(COMPOSE_FILES) up -d

down:
	docker compose $(COMPOSE_FILES) down

logs:
	docker compose $(COMPOSE_FILES) logs -f

# b) Caddy + Auth
up-auth:
	docker compose $(COMPOSE_FILES) --profile auth up -d

down-auth:
	docker compose $(COMPOSE_FILES) --profile auth down

logs-auth:
	docker compose $(COMPOSE_FILES) --profile auth logs -f

# c) All (add more profiles as needed)
up-all:
	docker compose $(COMPOSE_FILES) --profile auth up -d

down-all:
	docker compose $(COMPOSE_FILES) --profile auth down

logs-all:
	docker compose $(COMPOSE_FILES) --profile auth logs -f
