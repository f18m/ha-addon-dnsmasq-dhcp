ARG BUILD_FROM

# --- BACKEND BUILD
# About base image: we need to use a musl-based docker image since the actual HomeAssistant addon
# base image will be musl-based as well. This is required since we depend from "github.com/mattn/go-sqlite3"
# which is a CGO library; so that's why we select the -alpine variant
FROM golang:1.24-alpine AS builder

ARG TARGETPLATFORM

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
# needs to be compatible with all the architectures we support (armv7, aarch64, amd64, i386)
# and I couldn't find the "dart-sass" binary for all these architectures...
# we rather assume the transpiled version has been checked into git


# --- Actual ADDON layer

FROM $BUILD_FROM

# Add env
ENV LANG=C.UTF-8

# Setup base
RUN apk add --no-cache dnsmasq nginx-debug sqlite socat && mv /etc/nginx /etc/nginx-orig

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

LABEL org.opencontainers.image.source=https://github.com/f18m/ha-addon-dnsmasq-dhcp-server
