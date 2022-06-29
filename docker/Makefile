local-up:
	PLATFORM=$(shell uname -m) \
	ENVIROMENT=local \
	COMPOSE_DOCKER_CLI_BUILD=1 \
	docker compose -f docker-compose.yml -f local/docker-compose.override.yml up --build
.PHONY: local-up

local-down:
	docker compose -f docker-compose.yml -f local/docker-compose.override.yml down
.PHONY: local-down

HEALTHBOT_OVERRIDE=$(shell if [ ${HEALTHBOT_ENABLED} = "true" ]; then echo "-f deployed/docker-compose.healthbot.yml"; fi)

staging-up:
	PLATFORM=$(shell uname -m) \
	ENVIROMENT=deployed/staging \
	COMPOSE_DOCKER_CLI_BUILD=1 \
	docker compose -f docker-compose.yml -f deployed/docker-compose.observability.yml $(HEALTHBOT_OVERRIDE) up --build --remove-orphans
.PHONY: staging-up

staging-down:
	docker compose -f docker-compose.yml -f deployed/docker-compose.observability.yml $(HEALTHBOT_OVERRIDE)  down
.PHONY: staging-down