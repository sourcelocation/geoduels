variable "REGISTRY" {
  default = "ghcr.io/sourcelocation"
}

variable "TAG" {
  default = "latest"
}

variable "PLATFORMS" {
  default = "linux/arm64"
}

variable "APP_VERSION" {
  default = TAG
}

variable "GIT_SHA" {
  default = "dev"
}

// CI opts into GitHub Actions caching; local builds use BuildKit's own cache.
variable "CACHE_SCOPE" {
  default = ""
}

// Set to "true" to read the cache without writing it. Only main writes, since
// caches saved on PR or tag refs are invisible to later runs.
variable "CACHE_READONLY" {
  default = ""
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
      "discord-worker",
      "match-coordinator",
      "moderation-worker",
      "realtime-gateway",
      "gameplay-node",
    ]
  }
  context = "backend"
  dockerfile = "services/${service}/Dockerfile"
  tags = ["${REGISTRY}/geoduels-${service}:${TAG}"]
  cache-from = CACHE_SCOPE != "" ? ["type=gha,scope=${CACHE_SCOPE}-${service}"] : []
  cache-to = CACHE_SCOPE != "" && CACHE_READONLY != "true" ? ["type=gha,mode=max,scope=${CACHE_SCOPE}-${service}"] : []
}

target "web" {
  inherits = ["_common"]
  context = "web"
  dockerfile = "Dockerfile"
  tags = ["${REGISTRY}/geoduels-web:${TAG}"]
  args = {
    NEXT_PUBLIC_APP_VERSION = APP_VERSION
    NEXT_PUBLIC_GIT_SHA = GIT_SHA
  }
  cache-from = CACHE_SCOPE != "" ? ["type=gha,scope=${CACHE_SCOPE}-web"] : []
  cache-to = CACHE_SCOPE != "" && CACHE_READONLY != "true" ? ["type=gha,mode=max,scope=${CACHE_SCOPE}-web"] : []
}
