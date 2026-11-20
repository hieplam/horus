# The image of horus for the stack of compose.yaml (ADR 0029). One image for
# two containers: horus-api runs horus api, and horus-worker runs horus
# worker. compose.yaml builds it from the root of a clone:
#
#   docker compose build
#
# The build stage compiles the Linux program horus with CGO_ENABLED=0. The
# runtime stage holds only that program, the CA certificates (install_workflow
# downloads over HTTPS) and the time zones.

FROM golang:1.27.3-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG HORUS_VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X github.com/hieplam/horus/internal/version.Version=${HORUS_VERSION}" -o /out/horus ./cmd/horus

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 horus
COPY --from=build /out/horus /usr/local/bin/horus
USER horus
WORKDIR /home/horus
# The stack never starts ComfyUI: the owner starts it on the host.
ENV HORUS_AUTOSTART=0
CMD ["horus", "api", "-listen", "0.0.0.0:8190"]
