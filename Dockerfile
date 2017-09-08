FROM golang:1.9.0-alpine3.6 AS BUILD

MAINTAINER CMGS <ilskdw@gmail.com>

# make binary
RUN apk add --no-cache build-base musl-dev libgit2-dev git curl make \
    && curl https://glide.sh/get | sh \
    && go get -d github.com/projecteru2/core \
    && cd src/github.com/projecteru2/core \
    && make build \
    && ./eru-core --version

FROM alpine:3.6

MAINTAINER CMGS <ilskdw@gmail.com>

RUN mkdir /etc/eru/
RUN apk add --no-cache libgit2 && rm -rf /var/cache/apk/*
COPY --from=BUILD /go/src/github.com/projecteru2/core/eru-core /usr/bin/eru-core
COPY --from=BUILD /go/src/github.com/projecteru2/core/core.yaml.sample /etc/eru/core.yaml.sample
