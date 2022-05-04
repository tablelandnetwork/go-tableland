#!/bin/bash
set -euox pipefail

if [ "$#" -ne 3 ]; then
	echo "use $0 <gcloud-zoneid> <gcloud-vmname> <gcloud-project>"
	exit -1
fi

docker stop tablelandbuildimage || true; docker rm tablelandbuildimage || true;
DOCKER_TAG=$(date +%Y%m%d_%H%M%S)

docker build -f ../../local/postgres.Dockerfile -t tableland/postgres:$DOCKER_TAG $PWD/../..
docker run -d --name tablelandbuildimage -e POSTGRES_USER=admin -e POSTGRES_PASSWORD=admin -e PGDATA=/data tableland/postgres:$DOCKER_TAG

gcloud compute ssh \
--zone $1 $2  \
--project $3 \
--command="sudo su postgres -c 'pg_dumpall --clean --if-exists'" > dump.sql

until docker exec tablelandbuildimage pg_isready -U admin; do sleep 3; done
docker exec -i tablelandbuildimage psql -U admin < dump.sql
rm dump.sql

docker container commit tablelandbuildimage textile/tableland-postgres:$DOCKER_TAG
docker stop tablelandbuildimage
docker rm tablelandbuildimage

echo "Successfully created Docker image textile/tableland-postgres:$DOCKER_TAG"
