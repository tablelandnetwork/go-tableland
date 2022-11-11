# Validator Provisioner

The validator provisioner is a Terraform resource for creating an instance of a Tableland Validator in Google Cloud.

## First time setup

The first time you'll have to run `terraform init` to install the necessary plugins.

## Creating

```bash
TF_VAR_credentials_file={{GCP SERVICE ACCOUNT}} TF_VAR_user={{GCP USERNAME}} TF_VAR_vm_name={{INSTANCE NAME}} terraform apply
```

## Destroying

```bash
terraform destroy
```
