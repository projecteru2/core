FROM golang:alpine AS BUILD

# make binary
RUN apk add --no-cache build-base musl-dev libgit2-dev git curl make cmake python\
    && go get -d github.com/projecteru2/core
WORKDIR /go/src/github.com/projecteru2/core
RUN make build && ./eru-core --version

FROM alpine:latest

RUN mkdir /etc/eru/
LABEL ERU=1
RUN apk --no-cache add libcurl libssh2 libgit2 && rm -rf /var/cache/apk/*
COPY --from=BUILD /go/src/github.com/projecteru2/core/eru-core /usr/bin/eru-core
COPY --from=BUILD /go/src/github.com/projecteru2/core/core.yaml.sample /etc/eru/core.yaml.sample
