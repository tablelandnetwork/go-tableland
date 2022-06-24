# Local development with docker-compose

PLATFORM ?= $(shell uname -m)

up:
	sed "s/{{platform}}/$(PLATFORM)/g" docker-compose-dev.yml | COMPOSE_DOCKER_CLI_BUILD=1 docker-compose -f - up --build
.PHONY: up

down:
	docker-compose -f docker-compose-dev.yml down
.PHONY: down

# Note that -V indicates that we always start fresh.
kovan-staging-up:
	sed "s/{{platform}}/$(PLATFORM)/g" docker-compose-live.yml | \
	ENV_FILE=.env_kovan-staging \
	CONFIG_FILE=config-kovan-staging.json \
	COMPOSE_DOCKER_CLI_BUILD=1 \
	docker-compose -f - up --build -V
.PHONY: kovan-staging-up

kovan-staging-down:
	docker-compose -f docker-compose-live.yml down
.PHONY: kovan-staging-down