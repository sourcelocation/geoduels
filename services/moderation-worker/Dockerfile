# syntax=docker/dockerfile:1.7
FROM golang:1.26 AS build
WORKDIR /src
ARG TARGETOS
ARG TARGETARCH
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /out/moderation-worker ./services/moderation-worker

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/moderation-worker /moderation-worker
EXPOSE 8093
ENTRYPOINT ["/moderation-worker"]
