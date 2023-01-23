# Validator Provisioner

The validator provisioner is a Terraform resource for creating an instance of a Tableland Validator in Google Cloud.

## First time setup

The first time you'll have to run `terraform init` to install the necessary plugins.

## Creating

- Prepare needed files in this folder:
  - `.env_grafana` with credentials.
  - `.env_validator` with the corresponding values.
  - `.env_healthbot` with the corresponding values.
  - `grafana.db` if you want to copy the alerts from another environment (use `gcloud scp` to pull the file)
- Run

    ```bash
    TF_VAR_credentials_file=<credentials-file> TF_VAR_user=<gcloud-user> TF_VAR_vm_name=<vm-name> TF_VAR_gcp_project=<project-id>  TF_VAR_gcp_zone=<gcloud-zone>  TF_VAR_gcp_region=<gcloud-region>  TF_VAR_machine_type=<machine-type>  terraform apply
    ```

## Destroying

```bash
terraform destroy
```
