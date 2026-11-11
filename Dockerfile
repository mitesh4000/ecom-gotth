# syntax=docker/dockerfile:1.7

FROM golang:1.22-bookworm AS builder

ARG TEMPL_VERSION=v0.2.793
ARG TAILWIND_VERSION=v4.3.5
ARG TARGETARCH

WORKDIR /src

# Build-time tools are not present in the final image.
RUN go install github.com/a-h/templ/cmd/templ@${TEMPL_VERSION}

RUN set -eux; \
    arch="${TARGETARCH:-amd64}"; \
    case "$arch" in \
      amd64) asset="tailwindcss-linux-x64" ;; \
      arm64) asset="tailwindcss-linux-arm64" ;; \
      *) echo "unsupported build architecture: $arch" >&2; exit 1 ;; \
    esac; \
    curl -fsSL -o /usr/local/bin/tailwindcss \
      "https://github.com/tailwindlabs/tailwindcss/releases/download/${TAILWIND_VERSION}/${asset}"; \
    chmod +x /usr/local/bin/tailwindcss

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Generated Templ files and minified CSS are produced inside the build so the
# final binary contains exactly the assets from this source revision.
RUN templ generate \
    && tailwindcss -i ./styles/input.css -o ./static/css/app.css --minify \
    && CGO_ENABLED=0 GOOS=linux GOARCH="${TARGETARCH:-$(go env GOARCH)}" go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
    && CGO_ENABLED=0 GOOS=linux GOARCH="${TARGETARCH:-$(go env GOARCH)}" go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed \
    && mkdir -p /out/data/tmp /out/uploads \
    && chmod 1777 /out/data/tmp

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder --chown=65532:65532 /out/server /app/server
COPY --from=builder --chown=65532:65532 /out/seed /app/seed
COPY --from=builder --chown=65532:65532 /out/data /data
COPY --from=builder --chown=65532:65532 /out/uploads /uploads

ENV PORT=5000 \
    ENV=production \
    DATABASE_URL=/data/app.db \
    UPLOADS_DIR=/uploads \
    TMPDIR=/data/tmp

EXPOSE 5000
VOLUME ["/data", "/uploads"]

USER nonroot:nonroot

# Distroless has no shell or curl, so the binary probes its own /healthz route.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/app/server", "healthcheck"]

ENTRYPOINT ["/app/server"]
