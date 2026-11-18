FROM golang:1.26.8-alpine AS build
WORKDIR /src
COPY services/go.mod services/go.sum ./
RUN go mod download
COPY services/ .
ARG SERVICE=api-service
RUN CGO_ENABLED=0 go build -trimpath -o /app ./cmd/${SERVICE}
FROM alpine:3.22.1
RUN adduser -D -u 10001 app
COPY --from=build /app /app
COPY infrastructure/migrations /migrations
ENV MIGRATIONS_DIR=/migrations
USER app
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=5s CMD wget -q -O /dev/null http://localhost:8080/health/ready || exit 1
ENTRYPOINT ["/app"]
