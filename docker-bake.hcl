variable "REGISTRY" {
  default = "ghcr.io/sourcelocation"
}

variable "TAG" {
  default = "latest"
}

variable "PLATFORMS" {
  default = "linux/arm64"
}

variable "VERSION" {
  default = TAG
}

variable "REVISION" {
  default = "dev"
}

// Registry cache: "write" in releases, "read" in CI, empty locally.
variable "CACHE" {
  default = ""
}

variable "CACHE_TAG" {
  default = "buildcache"
}

group "default" {
  targets = ["backend", "web"]
}

target "_common" {
  platforms = split(",", PLATFORMS)
}

target "backend" {
  inherits = ["_common"]
  name = service
  matrix = {
    service = [
      "api",
      "gateway",
      "discord-worker",
      "moderation-worker",
      "gameplay-node",
    ]
  }
  context = "backend"
  dockerfile = "services/${service}/Dockerfile"
  tags = ["${REGISTRY}/geoduels-${service}:${TAG}"]
  cache-from = CACHE != "" ? ["type=registry,ref=${REGISTRY}/geoduels-${service}:${CACHE_TAG}"] : []
  cache-to = CACHE == "write" ? ["type=registry,ref=${REGISTRY}/geoduels-${service}:${CACHE_TAG},mode=max"] : []
}

target "web" {
  inherits = ["_common"]
  context = "web"
  dockerfile = "Dockerfile"
  tags = ["${REGISTRY}/geoduels-web:${TAG}"]
  args = {
    NEXT_PUBLIC_APP_VERSION = VERSION
    NEXT_PUBLIC_GIT_SHA = REVISION
  }
  cache-from = CACHE != "" ? ["type=registry,ref=${REGISTRY}/geoduels-web:${CACHE_TAG}"] : []
  cache-to = CACHE == "write" ? ["type=registry,ref=${REGISTRY}/geoduels-web:${CACHE_TAG},mode=max"] : []
}
