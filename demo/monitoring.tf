resource "kubernetes_namespace" "monitoring" {
  metadata {
    name = "monitoring"
  }
}

resource "kubernetes_secret" "cgr_dev_monitoring" {
  metadata {
    name      = "cgr-dev"
    namespace = kubernetes_namespace.monitoring.metadata[0].name
  }

  type = "kubernetes.io/dockerconfigjson"

  data = {
    ".dockerconfigjson" = jsonencode({
      auths = {
        "cgr.dev" = {
          username = var.cgr_username
          password = var.cgr_token
          auth     = base64encode("${var.cgr_username}:${var.cgr_token}")
        }
      }
    })
  }
}

resource "helm_release" "kube_prometheus_stack" {
  name       = "kps"
  repository = "https://prometheus-community.github.io/helm-charts"
  chart      = "kube-prometheus-stack"
  namespace  = kubernetes_namespace.monitoring.metadata[0].name

  set = [
    {
      name  = "global.imageRegistry"
      value = "cgr.dev"
    },
    {
      name  = "global.imagePullSecrets[0]"
      value = "cgr-dev"
    },
    {
      name  = "alertmanager.alertmanagerSpec.image.repository"
      value = "${var.organization}/prometheus-alertmanager"
    },
    {
      name  = "alertmanager.alertmanagerSpec.image.tag"
      value = "0.28.1"
    },
    {
      name  = "prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues"
      value = "false"
    },
    {
      name  = "prometheus.prometheusSpec.image.repository"
      value = "${var.organization}/prometheus"
    },
    {
      name  = "prometheus.prometheusSpec.image.tag"
      value = "3.4.1"
    },
    {
      name  = "prometheus-node-exporter.image.repository"
      value = "${var.organization}/prometheus-node-exporter"
    },
    {
      name  = "prometheus-node-exporter.image.tag"
      value = "1.9.0"
    },
    {
      name  = "kube-state-metrics.image.repository"
      value = "${var.organization}/kube-state-metrics"
    },
    {
      name  = "kube-state-metrics.image.tag"
      value = "latest"
    },
    {
      name  = "prometheusAdapter.image.repository"
      value = "${var.organization}/prometheus-adapter"
    },
    {
      name  = "grafana.image.repository"
      value = "${var.organization}/grafana"
    },
    {
      name  = "grafana.image.tag"
      value = "latest"
    },
    {
      name  = "grafana.image.pullSecrets[0].name"
      value = "cgr-dev"
    },
    {
      name  = "grafana.sidecar.image.repository"
      value = "${var.organization}/k8s-sidecar"
    },
    {
      name  = "prometheusOperator.image.repository"
      value = "${var.organization}/prometheus-operator"
    },
    {
      name  = "prometheusOperator.image.tag"
      value = "latest"
    },
    {
      name  = "prometheusOperator.admissionWebhooks.patch.image.repository"
      value = "${var.organization}/kube-webhook-certgen"
    },
    {
      name  = "prometheusOperator.admissionWebhooks.patch.image.tag"
      value = "latest"
    },
    {
      name  = "prometheusOperator.prometheusConfigReloader.image.repository"
      value = "${var.organization}/prometheus-config-reloader"
    },
    {
      name  = "prometheusOperator.prometheusConfigReloader.image.tag"
      value = "latest"
    }
  ]

  depends_on = [
    kubernetes_secret.cgr_dev_monitoring
  ]
}

resource "kubernetes_config_map" "grafana_dashboard_chainguard_adoption" {
  metadata {
    name      = "chainguard-adoption-dashboard"
    namespace = kubernetes_namespace.monitoring.metadata[0].name
    labels = {
      grafana_dashboard = "1"
    }
  }

  data = {
    "chainguard-adoption.json" = file("${path.module}/../dashboards/chainguard-adoption.json")
  }

  depends_on = [
    helm_release.kube_prometheus_stack
  ]
}
