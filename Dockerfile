FROM golang:1-bullseye AS builder

WORKDIR /workdir/
COPY . /workdir/

RUN apt-get update

RUN update-ca-certificates

RUN make build

FROM debian:bullseye-slim

RUN apt-get update && apt-get install -y git

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /workdir/bin/review ./usr/bin

COPY scripts/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# ENTRYPOINT ["/entrypoint.sh"]
