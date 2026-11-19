variable "REGISTRY" { default = "ghcr.io/sourcelocation" }
variable "TAG" { default = "dev" }
variable "VERSION" { default = TAG }
variable "REVISION" { default = "dev" }

# CI's registry cache, per image and architecture: image checks read it, releases write it.
variable "CACHE" { default = "" }
variable "ARCH" { default = "amd64" }

function "image" {
  params = [name]
  result = "${REGISTRY}/geoduels-${name}"
}

function "cache_from" {
  params = [name]
  result = CACHE == "" ? [] : ["type=registry,ref=${image(name)}:buildcache-${ARCH}"]
}

function "cache_to" {
  params = [name]
  result = CACHE == "write" ? ["type=registry,ref=${image(name)}:buildcache-${ARCH},mode=max"] : []
}

group "default" { targets = ["backend", "web"] }

target "backend" {
  name = service
  matrix = {
    service = ["api", "gateway", "discord-worker", "moderation-worker", "gameplay-node"]
  }
  context = "backend"
  dockerfile = "services/${service}/Dockerfile"
  tags = ["${image(service)}:${TAG}"]
  cache-from = cache_from(service)
  cache-to = cache_to(service)
}

target "web" {
  context = "web"
  tags = ["${image("web")}:${TAG}"]
  cache-from = cache_from("web")
  cache-to = cache_to("web")
  args = {
    NEXT_PUBLIC_APP_VERSION = VERSION
    NEXT_PUBLIC_GIT_SHA = REVISION
  }
}
