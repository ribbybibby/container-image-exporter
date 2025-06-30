resource "kubernetes_namespace" "container_image_exporter" {
  metadata {
    name = "container-image-exporter"
  }
}

resource "kubernetes_service_account" "container_image_exporter" {
  metadata {
    name      = "container-image-exporter"
    namespace = kubernetes_namespace.container_image_exporter.metadata[0].name
    labels = {
      "app.kubernetes.io/name" = "container-image-exporter"
    }
  }
}

resource "kubernetes_cluster_role" "container_image_exporter" {
  metadata {
    name = "container-image-exporter"
    labels = {
      "app.kubernetes.io/name" = "container-image-exporter"
    }
  }

  rule {
    api_groups = [""]
    resources  = ["pods"]
    verbs      = ["get", "list", "watch"]
  }

  rule {
    api_groups = [""]
    resources  = ["secrets", "serviceaccounts"]
    verbs      = ["get", "list"]
  }

  rule {
    api_groups = ["apps"]
    resources  = ["deployments", "statefulsets", "daemonsets"]
    verbs      = ["get", "list", "watch"]
  }

  rule {
    api_groups = ["batch"]
    resources  = ["jobs", "cronjobs"]
    verbs      = ["get", "list", "watch"]
  }
}

resource "kubernetes_cluster_role_binding" "container_image_exporter" {
  metadata {
    name = "container-image-exporter"
    labels = {
      "app.kubernetes.io/name" = "container-image-exporter"
    }
  }

  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "ClusterRole"
    name      = kubernetes_cluster_role.container_image_exporter.metadata[0].name
  }

  subject {
    kind      = "ServiceAccount"
    name      = kubernetes_service_account.container_image_exporter.metadata[0].name
    namespace = kubernetes_namespace.container_image_exporter.metadata[0].name
  }
}

resource "kubernetes_service" "container_image_exporter" {
  metadata {
    name      = "container-image-exporter"
    namespace = kubernetes_namespace.container_image_exporter.metadata[0].name
    labels = {
      "app.kubernetes.io/name" = "container-image-exporter"
    }
    annotations = {
      "prometheus.io/scrape" = "true"
      "prometheus.io/port"   = "8080"
      "prometheus.io/path"   = "/metrics"
    }
  }

  spec {
    selector = {
      "app.kubernetes.io/name" = "container-image-exporter"
    }

    port {
      name        = "metrics"
      port        = 8080
      target_port = 8080
      protocol    = "TCP"
    }

    port {
      name        = "health"
      port        = 8081
      target_port = 8081
      protocol    = "TCP"
    }
  }
}

resource "kubernetes_deployment" "container_image_exporter" {
  metadata {
    name      = "container-image-exporter"
    namespace = kubernetes_namespace.container_image_exporter.metadata[0].name
    labels = {
      "app.kubernetes.io/name" = "container-image-exporter"
    }
  }

  spec {
    replicas = 1

    selector {
      match_labels = {
        "app.kubernetes.io/name" = "container-image-exporter"
      }
    }

    template {
      metadata {
        labels = {
          "app.kubernetes.io/name" = "container-image-exporter"
        }
      }

      spec {
        service_account_name = kubernetes_service_account.container_image_exporter.metadata[0].name

        container {
          name              = "controller"
          image             = var.image
          image_pull_policy = "IfNotPresent"

          port {
            container_port = 8080
            name           = "metrics"
          }

          port {
            container_port = 8081
            name           = "health"
          }

          liveness_probe {
            http_get {
              path = "/healthz"
              port = 8081
            }
            initial_delay_seconds = 15
            period_seconds        = 20
          }

          readiness_probe {
            http_get {
              path = "/readyz"
              port = 8081
            }
            initial_delay_seconds = 5
            period_seconds        = 10
          }

          resources {
            limits = {
              cpu    = "500m"
              memory = "128Mi"
            }
            requests = {
              cpu    = "10m"
              memory = "64Mi"
            }
          }

          security_context {
            allow_privilege_escalation = false
            read_only_root_filesystem  = true
            run_as_non_root            = true
            run_as_user                = 65532
            capabilities {
              drop = ["ALL"]
            }
          }
        }
      }
    }
  }
}

resource "kubernetes_manifest" "container_image_exporter_service_monitor" {
  manifest = {
    apiVersion = "monitoring.coreos.com/v1"
    kind       = "ServiceMonitor"
    metadata = {
      name      = "container-image-exporter"
      namespace = kubernetes_namespace.monitoring.metadata[0].name
    }
    spec = {
      endpoints = [
        {
          path = "/metrics"
          port = "metrics"
        }
      ]
      namespaceSelector = {
        matchNames = [
          kubernetes_namespace.container_image_exporter.metadata[0].name
        ]
      }
      selector = {
        matchLabels = {
          "app.kubernetes.io/name" = "container-image-exporter"
        }
      }
    }
  }

  depends_on = [
    helm_release.kube_prometheus_stack
  ]
}
