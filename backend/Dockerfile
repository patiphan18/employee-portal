# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build
WORKDIR /src
# Dependencies first so a source-only change reuses the module layer.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget \
 && adduser -D -u 10001 portal \
 && mkdir -p /data/uploads \
 && chown -R portal:portal /data
COPY --from=build /out/server /usr/local/bin/server
USER portal
ENV UPLOAD_DIR=/data/uploads PORT=8000
EXPOSE 8000
HEALTHCHECK --interval=15s --timeout=3s --start-period=20s --retries=5 \
  CMD wget -qO- http://localhost:8000/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/server"]
