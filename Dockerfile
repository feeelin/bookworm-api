# Этап 1: сборка
FROM golang:1.26.4-bookworm AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=1 go build \
    -ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILD_TIME}" \
    -o /app .
RUN mkdir /data

# Этап 2: выполнение
FROM gcr.io/distroless/base-debian12:nonroot
COPY --from=builder /app /app
COPY --from=builder --chown=65532:65532 /data /data
WORKDIR /data
EXPOSE 8080
ENTRYPOINT ["/app"]