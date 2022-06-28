local-up:
	PLATFORM=$(shell uname -m) \
	ENVIROMENT=local \
	COMPOSE_DOCKER_CLI_BUILD=1 \
	docker compose -f docker-compose.yml -f local/docker-compose.override.yml up --build
.PHONY: local-up

local-down:
	docker compose -f docker-compose.yml -f local/docker-compose.override.yml down
.PHONY: local-down

HEALTHBOT_OVERRIDE=$(shell if [ ${HEALTHBOT_ENABLED} = "true" ]; then echo -n "-f deployed/docker-compose.healthbot.yml"; fi)

staging-up:
	PLATFORM=$(shell uname -m) \
	ENVIROMENT=deployed/staging \
	COMPOSE_DOCKER_CLI_BUILD=1 \
	docker compose -f docker-compose.yml -f deployed/docker-compose.observability.yml $(HEALTHBOT_OVERRIDE) up --build --remove-orphans
.PHONY: staging-up

staging-down:
	docker compose -f docker-compose.yml -f deployed/docker-compose.observability.yml $(HEALTHBOT_OVERRIDE)  down
.PHONY: staging-down


lolz:
    d=$(date +%s) \
    i=0 \
    for file in $(find $1  -name 'Dockerfile') do \
        i=$(( i + 1 )) \
        echo "patching timestamp for $file" \
        touch -d @$(( d + i )) "$file" \
    done
.PHONY: lolz
