#
# Main addon dockerfile
# Please note that this dockerfile is meant to be processed by the
# by the Home Assistant ghcr.io/home-assistant/amd64-builder
# see https://developers.home-assistant.io/docs/add-ons/configuration/
#

# --- BACKEND BUILD
# About base image: we need to use a musl-based docker image since the actual HomeAssistant addon
# base image will be musl-based as well. This is required since we depend from "github.com/mattn/go-sqlite3"
# which is a CGO library; so that's why we select the -alpine variant
FROM golang:1.26-alpine AS builder

# build go backend
WORKDIR /app/backend
COPY backend .
RUN --mount=type=cache,target=/root/.cache/apk \
    apk add build-base
RUN --mount=type=cache,target=/root/.cache/go \
    CGO_ENABLED=1 go build -o /backend .

# download frontend dependencies
WORKDIR /app/frontend
COPY frontend .
RUN apk add yarn bash && \
    yarn

# NOTE: 
# we don't transpile the SCSS->CSS in this docker build because this "builder" layer
# needs to be compatible with all the architectures we support (aarch64, amd64)
# and I couldn't find the "dart-sass" binary for all these architectures...
# we rather assume the transpiled version has been checked into git


# --- Actual ADDON layer

FROM ghcr.io/home-assistant/base:3.23

# Add env
ENV LANG=C.UTF-8

# Setup base
RUN apk add --no-cache nginx-debug sqlite socat && mv /etc/nginx /etc/nginx-orig

# Install dnsmasq
# A specific version is installed so it's clear what we ship in this HomeAssistant App.
# Check which version is available using:
#  docker run -ti --entrypoint=/bin/sh   ghcr.io/home-assistant/base:3.22
#  apk search dnsmasq
# See also https://thekelleys.org.uk/dnsmasq/CHANGELOG
RUN apk add --no-cache dnsmasq=2.91-r0

# Copy data
COPY rootfs /
COPY config.yaml /opt/bin/addon-config.yaml

# Copy web frontend HTML, CSS and JS files
COPY frontend/*.html /opt/web/templates/
COPY --from=builder /app/frontend/external-libs/*.js /opt/web/static/
COPY --from=builder /app/frontend/external-libs/*.css /opt/web/static/
COPY --from=builder /app/frontend/libs/*.css /opt/web/static/
COPY frontend/libs/*.js /opt/web/static/
COPY frontend/images/*.png /opt/web/static/

# Copy backend binary
COPY --from=builder /backend /opt/bin/

LABEL \
    org.opencontainers.image.title="Dnsmasq-DHCP App" \
    org.opencontainers.image.description="An Home Assistant app that runs dnsmasq both as DHCP server and DNS server." \
    org.opencontainers.image.source="https://github.com/f18m/ha-addon-dnsmasq-dhcp-server" \
    org.opencontainers.image.licenses="Apache License 2.0"
