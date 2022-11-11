terraform {
    required_providers {
        google = {
            source = "hashicorp/google"
            version = "4.37.0"
        }
    }
}

variable "vm_name" {
    type = string
}

variable "machine_type" {
    type = string
}

variable "gcp_project" {
    type = string
}

variable "gcp_region" {
    type = string
}

variable "gcp_zone" {
    type = string
}

variable "user" {
    type = string
}

variable "credentials_file" {
    type = string
}

provider "google" {
    credentials = file("${var.credentials_file}")

    project = "${var.gcp_project}"
    region  = "${var.gcp_region}"
    zone    = "${var.gcp_zone}"
}

resource "google_compute_instance" "validator" {
    name         = "${var.vm_name}"
    machine_type = "${var.machine_type}"
    tags         = ["https-server", "grafana", "ssh-vm"]
    deletion_protection = false

    boot_disk {
        initialize_params {
            image = "ubuntu-minimal-2204-jammy-v20221101"
            type = "pd-ssd"
            size = "50"
        }
    }
    
    network_interface {
        network = "default"
        access_config {}
    }

    service_account {
        scopes = ["service-control", "service-management", "storage-rw", "monitoring", "logging-write", "trace"]
    }

    provisioner "file" {
        source      = "bootstrap.sh"
        destination = "/tmp/bootstrap.sh"

        connection {
            type        = "ssh"
            user        = "${var.user}"
            timeout     = "500s"
            private_key = "${file("~/.ssh/google_compute_engine")}"
            host        = self.network_interface[0].access_config[0].nat_ip
        }
    }

    provisioner "file" {
        source      = ".env_validator"
        destination = "/tmp/.env_validator"

        connection {
            type        = "ssh"
            user        = "${var.user}"
            timeout     = "500s"
            private_key = "${file("~/.ssh/google_compute_engine")}"
            host        = self.network_interface[0].access_config[0].nat_ip
        }
    }

    provisioner "file" {
        source      = ".env_healthbot"
        destination = "/tmp/.env_healthbot"

        connection {
            type        = "ssh"
            user        = "${var.user}"
            timeout     = "500s"
            private_key = "${file("~/.ssh/google_compute_engine")}"
            host        = self.network_interface[0].access_config[0].nat_ip
        }
    }

    provisioner "file" {
        source      = ".env_grafana"
        destination = "/tmp/.env_grafana"

        connection {
            type        = "ssh"
            user        = "${var.user}"
            timeout     = "500s"
            private_key = "${file("~/.ssh/google_compute_engine")}"
            host        = self.network_interface[0].access_config[0].nat_ip
        }
    }

    provisioner "file" {
        source      = "grafana.db"
        destination = "/tmp/grafana.db"

        connection {
            type        = "ssh"
            user        = "${var.user}"
            timeout     = "500s"
            private_key = "${file("~/.ssh/google_compute_engine")}"
            host        = self.network_interface[0].access_config[0].nat_ip
        }
    }

    provisioner "remote-exec" {
        connection {
            type        = "ssh"
            user        = "${var.user}"
            timeout     = "500s"
            private_key = "${file("~/.ssh/google_compute_engine")}"
            host        = self.network_interface[0].access_config[0].nat_ip
        }

        inline = [
            "chmod +x /tmp/bootstrap.sh",
            "/tmp/bootstrap.sh",
        ]
    }
}
