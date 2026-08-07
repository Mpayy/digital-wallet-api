FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/worker ./cmd/worker

FROM alpine:3.20 AS api
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=builder /out/api ./api
USER appuser
EXPOSE 8080
ENTRYPOINT ["./api"]

FROM alpine:3.20 AS worker
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=builder /out/worker ./worker
USER appuser
ENTRYPOINT ["./worker"]