# Example

This is an example of running the exporter in a local `kind` cluster.

See [here](./chainguard) for a version of these steps that runs the components
with private Chainguard images.

## Requirements

You must have these tools installed:

- `docker`
- `helm`
- `kind`
- `kubectl`

## Steps

Execute these steps from this directory.

1. Create a local `kind` cluster.

```
kind create cluster
```

2. Install the Kube Prometheus Stack. Use the custom values in this repository.

```
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm upgrade --create-namespace --install kps prometheus-community/kube-prometheus-stack --namespace monitoring -f values.yaml
```

3. Build and install the exporter.

```
docker build -t container-image-exporter:dev .
kind load docker-image container-image-exporter:dev
kubectl apply -f ../deploy/manifests/container-image-exporter.yaml
kubectl apply -n monitoring -f servicemonitor.yaml
```

4. Deploy a sample workload.

```
kubectl run nginx --image=cgr.dev/chainguard/nginx
```

5. Port forward the Prometheus webserver and execute some queries at
   `http://localhost:9090`.

```
kubectl -n monitoring port-forward svc/kps-kube-prometheus-stack-prometheus 9090
```
