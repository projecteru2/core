FROM debian:sid-slim AS BUILD

MAINTAINER CMGS <ilskdw@gmail.com>

RUN apt update \
    && apt install -y golang-1.12 git libgit2-dev make \
    && git clone https://github.com/projecteru2/core.git
WORKDIR /core
RUN export PATH=/usr/lib/go-1.12/bin:$PATH \
    && make build \
    && ./eru-core --version

FROM debian:sid-slim

RUN mkdir /etc/eru/
LABEL ERU=1
RUN apt update \
    && apt install -y libgit2-27 libssh2-1 \
    && rm -rf /var/lib/apt/lists/*
COPY --from=BUILD /core/eru-core /usr/bin/eru-core
COPY --from=BUILD /core/core.yaml.sample /etc/eru/core.yaml.sample
