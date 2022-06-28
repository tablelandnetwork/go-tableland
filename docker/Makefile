dev-up:
	PLATFORM=$(shell uname -m) \
	ENVIROMENT=dev \
	ENV_FILE=api/.env_validator \
	COMPOSE_DOCKER_CLI_BUILD=1 docker-compose -f docker-compose.yml -f dev/docker-compose.override.yml up --build
.PHONY: up

dev-down:
	docker-compose -f docker-compose.yml -f dev/docker-compose.override.yml down
.PHONY: down

staging-up:
	PLATFORM=$(shell uname -m) \
	ENVIROMENT=deployed/staging \
	ENV_FILE=api/.env_validator \
	COMPOSE_DOCKER_CLI_BUILD=1 \
	docker-compose -f docker-compose.yml -f deployed/docker-compose.override.yml up --build
.PHONY: kovan-staging-up

staging-down:
	docker-compose -f docker-compose.yml down
.PHONY: staging-down