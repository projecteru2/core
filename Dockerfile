FROM golang:1.10.3-alpine3.7 AS BUILD

MAINTAINER CMGS <ilskdw@gmail.com>

ENV LIBGIT2VERSION 0.26.4

# make binary
RUN apk add --no-cache build-base musl-dev libgit2-dev git curl make cmake python\
    && curl https://glide.sh/get | sh \
    && go get -d github.com/projecteru2/core
RUN wget -c https://github.com/libgit2/libgit2/archive/v$LIBGIT2VERSION.tar.gz -O - | tar -xz
RUN cd libgit2-$LIBGIT2VERSION && cmake . && make && cp libgit2.so* /tmp && make install
WORKDIR /go/src/github.com/projecteru2/core
RUN make build && ./eru-core --version

FROM alpine:3.7

MAINTAINER CMGS <ilskdw@gmail.com>

RUN mkdir /etc/eru/
LABEL ERU=1 version=latest
RUN apk --no-cache add libcurl libssh2 && rm -rf /var/cache/apk/*
COPY --from=BUILD /tmp/libgit2.so.26 /usr/lib/libgit2.so.26
COPY --from=BUILD /tmp/libgit2.so.0.26.4 /usr/lib/libgit2.so.0.26.4
COPY --from=BUILD /go/src/github.com/projecteru2/core/eru-core /usr/bin/eru-core
COPY --from=BUILD /go/src/github.com/projecteru2/core/core.yaml.sample /etc/eru/core.yaml.sample
