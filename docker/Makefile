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
	BOOTSTRAP_BACKUP_URL=${BOOTSTRAP_BACKUP_URL} \
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
	BOOTSTRAP_BACKUP_URL=${BOOTSTRAP_BACKUP_URL} \
	docker compose -f docker-compose.yml -f deployed/docker-compose.observability.yml $(HEALTHBOT_OVERRIDE) up --build --remove-orphans --detach
.PHONY: testnet-up

testnet-down:
	PLATFORM=$(shell uname -m) \
	USER=$(shell id -u) \
	ENVIRONMENT=deployed/testnet \
	docker compose -f docker-compose.yml -f deployed/docker-compose.observability.yml $(HEALTHBOT_OVERRIDE)  down
.PHONY: testnet-down

mainnet-up:
	PLATFORM=$(shell uname -m) \
	USER=$(shell id -u) \
	ENVIRONMENT=deployed/mainnet \
	COMPOSE_DOCKER_CLI_BUILD=1 \
	BOOTSTRAP_BACKUP_URL=${BOOTSTRAP_BACKUP_URL} \
	docker compose -f docker-compose.yml -f deployed/docker-compose.observability.yml $(HEALTHBOT_OVERRIDE) up --build --remove-orphans --detach
.PHONY: mainnet-up

mainnet-down:
	PLATFORM=$(shell uname -m) \
	USER=$(shell id -u) \
	ENVIRONMENT=deployed/mainnet \
	docker compose -f docker-compose.yml -f deployed/docker-compose.observability.yml $(HEALTHBOT_OVERRIDE)  down
.PHONY: mainnet-down
