local-up:
	PLATFORM=$(shell uname -m) \
	ENVIROMENT=local \
	ENV_FILE=api/.env_validator \
	COMPOSE_DOCKER_CLI_BUILD=1 docker-compose -f docker-compose.yml -f local/docker-compose.override.yml up --build
.PHONY: local-up

local-down:
	docker-compose -f docker-compose.yml -f local/docker-compose.override.yml down
.PHONY: local-down

staging-up:
	PLATFORM=$(shell uname -m) \
	ENVIROMENT=deployed/staging \
	ENV_FILE=api/.env_validator \
	COMPOSE_DOCKER_CLI_BUILD=1 \
	docker-compose -f docker-compose.yml -f deployed/docker-compose.override.yml up --build
.PHONY: staging-up

staging-down:
	docker-compose -f docker-compose.yml down
.PHONY: staging-down