local-up:
	PLATFORM=$(shell uname -m) \
	ENVIRONMENT=local \
	COMPOSE_DOCKER_CLI_BUILD=1 \
	docker compose -f docker-compose.yml -f local/docker-compose.override.yml up --build
.PHONY: local-up

local-down:
	docker compose -f docker-compose.yml -f local/docker-compose.override.yml down
.PHONY: local-down

HEALTHBOT_OVERRIDE=$(shell if [ ${HEALTHBOT_ENABLED} = "true" ]; then echo "-f deployed/docker-compose.healthbot.yml"; fi)

staging-up:
	PLATFORM=$(shell uname -m) \
	USER=$(shell id -u) \
	ENVIRONMENT=deployed/staging \
	COMPOSE_DOCKER_CLI_BUILD=1 \
	FROM_BACKUP=$(shell if [ ${FROM_BACKUP} = "true" ]; then echo "true"; else echo "false"; fi) \
	docker compose -f docker-compose.yml -f deployed/docker-compose.observability.yml $(HEALTHBOT_OVERRIDE) up --build --remove-orphans --detach
.PHONY: staging-up

staging-down:
	PLATFORM=$(shell uname -m) \
	USER=$(shell id -u) \
	ENVIRONMENT=deployed/staging \
	docker compose -f docker-compose.yml -f deployed/docker-compose.observability.yml $(HEALTHBOT_OVERRIDE)  down
.PHONY: staging-down

testnet-up:
	PLATFORM=$(shell uname -m) \
	USER=$(shell id -u) \
	ENVIRONMENT=deployed/testnet \
	COMPOSE_DOCKER_CLI_BUILD=1 \
	FROM_BACKUP=$(shell if [ ${FROM_BACKUP} = "true" ]; then echo "true"; else echo "false"; fi) \
	docker compose -f docker-compose.yml -f deployed/docker-compose.observability.yml $(HEALTHBOT_OVERRIDE) up --build --remove-orphans --detach
.PHONY: testnet-up

testnet-down:
	PLATFORM=$(shell uname -m) \
	USER=$(shell id -u) \
	ENVIRONMENT=deployed/testnet \
	docker compose -f docker-compose.yml -f deployed/docker-compose.observability.yml $(HEALTHBOT_OVERRIDE)  down
.PHONY: testnet-down