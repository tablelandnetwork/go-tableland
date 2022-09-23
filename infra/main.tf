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

variable "user" {
    type = string
}

variable "credentials_file" {
    type = string
}

provider "google" {
    credentials = file("${var.credentials_file}")

    project = "textile-310716"
    region  = "us-west1"
    zone    = "us-west1-b"
}

resource "google_compute_instance" "validator" {
    name         = "${var.vm_name}"
    machine_type = "e2-standard-2"
    tags         = ["https-server", "grafana", "ssh-vm"]
    deletion_protection = false

    boot_disk {
        initialize_params {
            image = "ubuntu-2204-jammy-v20220902"
            type = "pd-ssd"
            size = "30"
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
