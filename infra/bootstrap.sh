#!/bin/bash

sudo DEBIAN_FRONTEND=noninteractive apt-get -y update
sudo DEBIAN_FRONTEND=noninteractive apt-get -y upgrade  
sudo DEBIAN_FRONTEND=noninteractive apt install -y git vim jq cron make
sudo DEBIAN_FRONTEND=noninteractive apt-get -y install \
    ca-certificates \
    curl \
    gnupg \
    lsb-release
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

sudo apt-get update
sudo DEBIAN_FRONTEND=noninteractive apt-get -y install docker-ce docker-ce-cli containerd.io docker-compose-plugin

sudo touch /etc/docker/daemon.json
sudo bash -c 'cat > /etc/docker/daemon.json <<- EOM
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "10m",
    "max-file": "10"
  }
}
EOM'

sudo service docker restart

sudo useradd -m validator
sudo usermod -aG docker validator

curl -sSO https://dl.google.com/cloudagents/add-google-cloud-ops-agent-repo.sh
sudo DEBIAN_FRONTEND=noninteractive bash add-google-cloud-ops-agent-repo.sh --also-install

sudo bash -c 'cat > /etc/google-cloud-ops-agent/config.yaml <<- EOM
logging:
  receivers:
    docker:
      type: files
      include_paths: [/var/lib/docker/containers/*/*-json.log]
      record_log_file_path: true
  processors:
    parse_message:
      type: parse_json
      field: message
    severity:
      type: modify_fields
      fields:
        severity:
          copy_from: jsonPayload.severity
    parse_log:
      type: parse_json
      field: log
  service:
    pipelines:
      default_pipeline:
        receivers: [syslog, docker]
        processors: [parse_message, parse_log, severity]
EOM'

sudo service google-cloud-ops-agent restart

sudo su - validator -c 'git clone https://github.com/tablelandnetwork/go-tableland.git /home/validator/go-tableland'
sudo su - validator -c 'cat /tmp/.env_validator > /home/validator/go-tableland/docker/deployed/testnet/api/.env_validator'
sudo su - validator -c 'cat /tmp/.env_grafana > /home/validator/go-tableland/docker/deployed/testnet/grafana/.env_grafana'
sudo su - validator -c 'cat /tmp/.env_healthbot > /home/validator/go-tableland/docker/deployed/testnet/healthbot/.env_healthbot'
sudo su - validator -c 'cat /tmp/grafana.db > /home/validator/go-tableland/docker/deployed/testnet/grafana/data/grafana.db'

#sudo su - validator -c 'cd ~/go-tableland/docker && make testnet-up'

sudo rm /tmp/.env_grafana
sudo rm /tmp/.env_validator
sudo rm /tmp/.env_healthbot
sudo rm /tmp/grafana.db
