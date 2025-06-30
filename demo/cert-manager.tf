resource "kubernetes_namespace" "cert_manager" {
  metadata {
    name = "cert-manager"
  }
}

resource "kubernetes_secret" "cgr_dev_cert_manager" {
  metadata {
    name      = "cgr-dev"
    namespace = kubernetes_namespace.cert_manager.metadata[0].name
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

resource "helm_release" "cert_manager" {
  name       = "cert-manager"
  repository = "https://charts.jetstack.io"
  chart      = "cert-manager"
  namespace  = kubernetes_namespace.cert_manager.metadata[0].name

  set = [
    {
      name  = "global.imagePullSecrets[0].name"
      value = "cgr-dev"
    },
    {
      name  = "installCRDs"
      value = "true"
    },
    {
      name  = "image.repository"
      value = "cgr.dev/${var.organization}/cert-manager-controller"
    },
    {
      name  = "image.tag"
      value = "latest"
    },
    //{
    //  name  = "webhook.image.repository"
    //  value = "cgr.dev/${var.organization}/cert-manager-webhook"
    //},
    //{
    //  name  = "webhook.image.tag"
    //  value = "latest"
    //},
    {
      name  = "cainjector.image.repository"
      value = "cgr.dev/${var.organization}/cert-manager-cainjector"
    },
    {
      name  = "cainjector.image.tag"
      value = "latest"
    }
  ]

  depends_on = [
    kubernetes_secret.cgr_dev_cert_manager
  ]
}
