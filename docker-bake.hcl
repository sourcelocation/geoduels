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

// The build cache lives in the registry (<image>:buildcache), so every run shares it. "write" (release
// builds) reads and writes it, "read" (CI's image check) only reads it; empty, as in local builds,
// leaves BuildKit to its own cache.
variable "CACHE" {
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
  cache-from = CACHE != "" ? ["type=registry,ref=${REGISTRY}/geoduels-${service}:buildcache"] : []
  cache-to = CACHE == "write" ? ["type=registry,ref=${REGISTRY}/geoduels-${service}:buildcache,mode=max"] : []
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
  cache-from = CACHE != "" ? ["type=registry,ref=${REGISTRY}/geoduels-web:buildcache"] : []
  cache-to = CACHE == "write" ? ["type=registry,ref=${REGISTRY}/geoduels-web:buildcache,mode=max"] : []
}
