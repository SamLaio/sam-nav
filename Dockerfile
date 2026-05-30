FROM golang:1.22-alpine AS sam-nav-build

WORKDIR /src

COPY ./site/go.mod ./
RUN go mod download

COPY ./site ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/sam-nav .

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata \
    && mkdir -p /app/data

COPY --from=sam-nav-build /out/sam-nav /usr/local/bin/sam-nav

ENV SAM_NAV_PORT=6412 \
    SAM_NAV_DATA_DIR=/app/data \
    SAM_NAV_DB_PATH=/app/data/nav.sqlite

EXPOSE 6412

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- "http://127.0.0.1:${SAM_NAV_PORT}/healthz" >/dev/null || exit 1

ENTRYPOINT ["sam-nav"]
