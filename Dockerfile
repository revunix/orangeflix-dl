# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/orangeflix-dl ./cmd/orangeflix-dl

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
COPY --from=build /out/orangeflix-dl /usr/bin/orangeflix-dl

ENV ORANGEFLIX_DOWNLOADS_DIR=/data/downloads
ENV XDG_STATE_HOME=/data
# The credentials file lives at /data/orangeflix-dl/credentials, on the volume.
ENV XDG_CONFIG_HOME=/data
WORKDIR /data

ENTRYPOINT ["/usr/bin/orangeflix-dl"]
