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


sudo -u validator  -i bash <<-'EOF'
export TBLENV=$(uname -n | cut -d '-' -f 3)

git clone https://github.com/tablelandnetwork/go-tableland.git ~/go-tableland
cd ~/go-tableland && git checkout jsign/testnetmainnetsplit

cat /tmp/.env_validator > ~/go-tableland/docker/deployed/${TBLENV}/api/.env_validator
cat /tmp/.env_grafana > ~/go-tableland/docker/deployed/${TBLENV}/grafana/.env_grafana
cat /tmp/.env_healthbot > ~/go-tableland/docker/deployed/${TBLENV}/healthbot/.env_healthbot
cat /tmp/grafana.db > ~/go-tableland/docker/deployed/${TBLENV}/grafana/data/grafana.db

mkdir ~/.ssh && chmod 700 ~/.ssh
echo "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEehs1xfBMLKpwV4sAIko++GQgauYXf5SNY4tl9ArTOG ops@textile.io" > ~/.ssh/authorized_keys

crontab - <<-MOF
0 0 * * FRI /usr/bin/docker system prune --volumes -f  >> /home/validator/cronrun 2>&1
10 * * * * gsutil rsync /home/validator/go-tableland/docker/deployed/${TBLENV}/api/backups/ gs://tableland-${TBLENV}/backups/ > /home/validator/gsutil.log 2>&1
10 * * * * gsutil cp "ls -A -1 /home/validator/go-tableland/docker/deployed/${TBLENV}/api/backups/*.* | tail -n 1" gs://tableland-${TBLENV}/backups/tbl_backup_latest.db.zst > /home/validator/gsutil.log 2>&1
MOF

EOF

#sudo su - validator -c 'cd ~/go-tableland/docker && make ${TBLENV}-up'

sudo rm /tmp/.env_grafana
sudo rm /tmp/.env_validator
sudo rm /tmp/.env_healthbot
sudo rm /tmp/grafana.db
