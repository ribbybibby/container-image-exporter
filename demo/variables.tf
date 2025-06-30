variable "cgr_username" {
  description = "The username for cgr.dev"
  type        = string
  default     = "_token"
}

variable "cgr_token" {
  description = "The Chainguard token for pulling images from cgr.dev"
  type        = string
  sensitive   = true
}

variable "organization" {
  description = "The Chainguard organization name"
  type        = string
}

variable "image" {
  description = "Container image for the container-image-exporter"
  type        = string
}
