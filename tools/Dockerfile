# Builder Image
FROM golang:1.23-alpine

ENV GITHUB_TOKEN=""
ENV GPG_PRIVATE_KEY=""
ENV GPG_FINGERPRINT=""
ENV PASSPHRASE=""

RUN apk add --no-cache \
    make \
    git \
    curl \
    gpg \
    gpg-agent

RUN go install github.com/goreleaser/goreleaser/v2@v2.4.8

CMD ["/bin/sh"]
