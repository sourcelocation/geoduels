# Development only (compose.dev.yaml): Air rebuilds and restarts each service from the bind-mounted
# repository on every save (services/<service>/.air.toml).
FROM golang:1.26.9-alpine
RUN go install github.com/air-verse/air@v1.67.4
ENV CGO_ENABLED=0
WORKDIR /src/backend
ENTRYPOINT ["air"]
