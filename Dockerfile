FROM golang:1.26.4 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 go build \
    -ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILD_TIME}" \
    -o /app .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /app /app
EXPOSE 8080
ENTRYPOINT ["/app"]