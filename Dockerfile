FROM golang:alpine AS BUILD

# make binary
RUN apk add --no-cache build-base musl-dev git curl make cmake
RUN git clone https://github.com/projecteru2/core.git /go/src/github.com/projecteru2/core
WORKDIR /go/src/github.com/projecteru2/core
ARG KEEP_SYMBOL
RUN make build && ./eru-core --version

FROM alpine:latest

RUN mkdir /etc/eru/
LABEL ERU=1
RUN apk --no-cache add libcurl libssh2 && rm -rf /var/cache/apk/*
COPY --from=BUILD /go/src/github.com/projecteru2/core/eru-core /usr/bin/eru-core
COPY --from=BUILD /go/src/github.com/projecteru2/core/core.yaml.sample /etc/eru/core.yaml.sample
