# Tableland Validator Node Helm Chart

This Helm chart installs a Tableland Validator Node in a Kubernetes cluster.

## Prerequisites

- Kubernetes 1.18+
- Helm 3.0+

## Get Repo Info

```shell
helm repo add [repo-name] [repo-url]
helm repo update
```


# Install Chart

```shell
helm install [release-name] [chart] -f values.yaml
```


# Uninstall Chart
```shell
helm uninstall [release-name]
```

Configuration
-------------

The following table lists the configurable parameters of the Tableland Validator Node chart and their default values, specified in `values.yaml`.

| Parameter | Description | Default |
| --- | --- | --- |
| `fullnameOverride` | Override the full resource names | `""` |
| `image` | Tableland image | `textile/tableland` |
| `imageTag` | Image tag | `"v1.8.1-beta-3"` |
| `imagePullPolicy` | Image pull policy | `"IfNotPresent"` |
| `imagePullSecrets` | Specify image pull secrets | `[]` |
| `httpPort` | Http port of the application | `8080` |
| `httpsPort` | Https port of the application | `8443` |
| `metricsPort` | Metrics port of the application | `8888` |
| `...` | ... | ... |

You can specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example,

```shell 
helm install [release-name] [repo-name]/tableland-validator-node --set imagePullPolicy=Always
```

Alternatively, a YAML file that specifies the values for the parameters can be provided while installing the chart. For example,


```shell
helm install [release-name] [repo-name]/tableland-validator-node -f values.yaml
``````

### Image

| Parameter | Description | Default |
| --- | --- | --- |
| `image.repository` | Tableland image name | `textile/tableland` |
| `image.tag` | Image tag | `v1.8.1-beta-3` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `imagePullSecrets` | Specify image pull secrets | `[]` |

### Pod Settings

| Parameter | Description | Default |
| --- | --- | --- |
| `podAnnotations` | Annotations for pods | `{}` |
| `resources` | Resource limits and requests | `{}` |
| `terminationGracePeriod` | Time to wait for clean shutdown | `120` |
| `...` | ... | ... |

### Configurations

Specify configurations for Tableland in `config` key. You can edit the configuration provided to the node under the values.yaml config key.

### Configurations

### Extra Environment Variables

`extraEnvs` provides a way to add extra environment variables. Use this to insert additional secret environment variables.

### Ingress

| Parameter | Description | Default |
| --- | --- | --- |
| `ingress.enabled` | Enables Ingress | `false` |
| `ingress.className` | Ingress Class Name | `nginx` |
| `ingress.hosts` | Ingress accepted hostnames | `[]` |
| `ingress.annotations` | Ingress annotations | `{}` |
| `ingress.tls` | Ingress TLS configuration | `[]` |

### RBAC settings, Persistence, etc.

The rest of the parameters (RBAC settings, Persistence, Extra Containers, etc.) can be found in the provided `values.yaml` file. Ensure to review and tailor them according to your use-case.

### Note

Ensure to review and customize the `values.yaml` file as per your deployment strategy to provide specific settings for environment variables, resources, etc.

# Configuration

The following table lists the configurable parameters of the Tableland Validator Node chart and their default values, specified in values.yaml.