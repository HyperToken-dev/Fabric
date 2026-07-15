FROM golang:1.26.4 AS builder

ARG SQLC_VERSION=v1.30.0
ARG BUF_VERSION=v1.57.2

WORKDIR /src

RUN apt-get update && \
    apt-get install -y --no-install-recommends make ca-certificates && \
    rm -rf /var/lib/apt/lists/*

RUN go install github.com/sqlc-dev/sqlc/cmd/sqlc@${SQLC_VERSION} && \
    go install github.com/bufbuild/buf/cmd/buf@${BUF_VERSION}

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN make generate
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gateway ./cmd/gateway

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S fabric && \
    adduser -S -G fabric fabric

WORKDIR /app

COPY --from=builder /out/gateway ./gateway
COPY configs ./configs
COPY db/migrations ./db/migrations

RUN mkdir -p /app/logs && chown -R fabric:fabric /app

USER fabric

EXPOSE 3002 9090

ENTRYPOINT ["./gateway"]
