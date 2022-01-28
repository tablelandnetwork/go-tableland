# Local development with docker-compose

PLATFORM ?= $(shell uname -m)

up:
	sed "s/{{platform}}/$(PLATFORM)/g" docker-compose-dev.yml | COMPOSE_DOCKER_CLI_BUILD=1 docker-compose -f - up --build
.PHONY: up

down:
	docker-compose -f docker-compose-dev.yml down
.PHONY: down

psql:
	docker-compose -f docker-compose-dev.yml exec api psql postgresql://dev_user:dev_password@database/dev_database
.PHONY: psql