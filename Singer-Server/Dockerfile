FROM golang:latest AS builder

WORKDIR /app
ADD . /app
RUN make build

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
RUN echo "UTC" >  /etc/timezone
COPY --from=builder /app/bin/clickhouse_sinker /usr/local/bin/clickhouse_sinker
COPY --from=builder /app/docker/dev.json /usr/local/bin/dev.json

VOLUME /usr/local/bin/dev.json

ENTRYPOINT ["/usr/local/bin/clickhouse_sinker", "--local-cfg-file", "/usr/local/bin/dev.json"]
