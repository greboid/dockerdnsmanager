FROM ghcr.io/greboid/dockerfiles/golang:latest as builder

WORKDIR /app
COPY . /app
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -gcflags=./dontoptimizeme=-N -ldflags=-s -o /app/main . && \
    find /app -exec touch --date=@0 {} \;
RUN mkdir /data

FROM ghcr.io/greboid/dockerfiles/base:latest

COPY --from=builder --chown=65532 /data /data

COPY --from=builder /app/main /containermonitor
CMD ["/containermonitor"]
