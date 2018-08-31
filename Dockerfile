FROM golang:1.10.3-alpine3.8 AS BUILD

MAINTAINER CMGS <ilskdw@gmail.com>

# make binary
RUN apk add --no-cache build-base musl-dev libgit2-dev git curl make cmake python\
    && curl https://glide.sh/get | sh \
    && go get -d github.com/projecteru2/core
WORKDIR /go/src/github.com/projecteru2/core
RUN make build && ./eru-core --version

FROM alpine:3.8

MAINTAINER CMGS <ilskdw@gmail.com>

RUN mkdir /etc/eru/
LABEL ERU=1
RUN apk --no-cache add libcurl libssh2 libgit2 && rm -rf /var/cache/apk/*
COPY --from=BUILD /go/src/github.com/projecteru2/core/eru-core /usr/bin/eru-core
COPY --from=BUILD /go/src/github.com/projecteru2/core/core.yaml.sample /etc/eru/core.yaml.sample
