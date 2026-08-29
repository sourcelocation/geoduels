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
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /out/discord-worker ./services/discord-worker

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/discord-worker /discord-worker
EXPOSE 8094
ENTRYPOINT ["/discord-worker"]
