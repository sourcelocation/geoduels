variable "REGISTRY" { default = "ghcr.io/sourcelocation" }
variable "TAG" { default = "dev" }
variable "VERSION" { default = TAG }
variable "REVISION" { default = "dev" }

group "default" { targets = ["backend", "web"] }

target "backend" {
  name = service
  matrix = {
    service = ["api", "gateway", "discord-worker", "moderation-worker", "gameplay-node"]
  }
  context = "backend"
  dockerfile = "services/${service}/Dockerfile"
  tags = ["${REGISTRY}/geoduels-${service}:${TAG}"]
}

target "web" {
  context = "web"
  tags = ["${REGISTRY}/geoduels-web:${TAG}"]
  args = {
    NEXT_PUBLIC_APP_VERSION = VERSION
    NEXT_PUBLIC_GIT_SHA = REVISION
  }
}
