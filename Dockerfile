ARG SAM_NAV_BUILD_HASH=""

FROM golang:1.26.3-alpine AS sam-nav-build

WORKDIR /src

COPY ./site/go.mod ./site/go.sum ./
RUN go mod download

COPY ./site ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/sam-nav .

FROM alpine:3.20

ARG SAM_NAV_BUILD_HASH=""

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata \
    && mkdir -p /app/data

COPY --from=sam-nav-build /out/sam-nav /usr/local/bin/sam-nav

ENV SAM_NAV_PORT=80 \
    SAM_NAV_DATA_DIR=/app/data \
    SAM_NAV_DB_PATH=/app/data/nav.sqlite \
    SAM_NAV_VERSION=v0.2 \
    SAM_NAV_BUILD_HASH=${SAM_NAV_BUILD_HASH}

LABEL org.opencontainers.image.version="v0.2"

EXPOSE 80

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- "http://127.0.0.1:${SAM_NAV_PORT}/healthz" >/dev/null || exit 1

ENTRYPOINT ["sam-nav"]
