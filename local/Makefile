# Local development with docker-compose

PLATFORM ?= $(shell uname -m)

up:
	sed "s/{{platform}}/$(PLATFORM)/g" docker-compose-dev.yml | COMPOSE_DOCKER_CLI_BUILD=1 docker-compose -f - up --build
.PHONY: up

down:
	docker-compose -f docker-compose-dev.yml down
.PHONY: down

staging-up:
	sed "s/{{platform}}/$(PLATFORM)/g" docker-compose-live.yml | \
	ENV_FILE=api/.env_validator \
	ENVIROMENT=staging \
	COMPOSE_DOCKER_CLI_BUILD=1 \
	docker-compose -f - up --build
.PHONY: kovan-staging-up

kovan-staging-down:
	docker-compose -f docker-compose-live.yml down
.PHONY: kovan-staging-down