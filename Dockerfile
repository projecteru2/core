FROM golang:1.27-alpine AS build

RUN apk add --no-cache git make
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG KEEP_SYMBOL
RUN make build && ./eru-core --version

FROM alpine:3.22

LABEL ERU=1
RUN mkdir -p /etc/eru
COPY --from=build /src/eru-core /usr/bin/eru-core
COPY --from=build /src/core.yaml.sample /etc/eru/core.yaml.sample
