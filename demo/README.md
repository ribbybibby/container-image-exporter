# Demo

This is an example of using the exporter to track adoption of Chainguard images
in a Kubernetes cluster.

## Requirements

You must have these tools installed:

- `chainctl`
- `docker`
- `helm`
- `kind`
- `kubectl`

And access to these images in your Chainguard organization:

- `prometheus-alertmanager`
- `prometheus`
- `prometheus-node-exporter`
- `kube-state-metrics`
- `prometheus-adapter`
- `grafana`
- `k8s-sidecar`
- `prometheus-operator`
- `prometheus-config-reloader`

## Steps

Execute these steps from this directory.

1. Create a `kind` cluster.

```
kind create cluster
```

2. Create a `.tfvars` file. The token this creates is valid for an hour.

```
cat <<EOF > terraform.tfvars
cgr_token = "$(chainctl auth token --audience=cgr.dev)"
organization = "org-name"
EOF
```

3. Apply the terraform module.

```
terraform init
terraform apply -var-file=terraform.tfvars
```

4. Port forward Grafana. Login at `localhost:3000` with the user `admin` and the
   password `prom-operator`.

```
kubectl -n monitoring port-forward svc/kps-grafana 3000:80
```
