FROM centos

MAINTAINER CMGS <ilskdw@gmail.com>

RUN yum -y update && \
    yum -y groupinstall "Development Tools" && \
    yum -y install NetworkManager sudo which wget ruby rubygems ruby-devel && \
    yum -y install http://cbs.centos.org/kojifiles/packages/http-parser/2.7.1/5.el7/x86_64/http-parser-2.7.1-5.el7.x86_64.rpm

ADD libgit2*.rpm /tmp/
RUN yum -y install /tmp/libgit2*.rpm && \
    rm -rf /tmp/libgit2*.rpm && \
    localedef -c --force --quiet -i zh_CN -f UTF-8 zh_CN.UTF-8 && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    sed -i -e "s/Defaults    requiretty.*/ #Defaults    requiretty/g" /etc/sudoers && \
    gem install fpm

ENV GOPATH /.go 
ENV GOBIN /.go/bin
ENV PATH $PATH:/usr/local/go/bin:$GOPATH/bin
ENV GOVERSION 1.9
RUN wget https://storage.googleapis.com/golang/go$GOVERSION.linux-amd64.tar.gz && \
    tar -C /usr/local -xvzf go$GOVERSION.linux-amd64.tar.gz && \
    rm -rf go$GOVERSION.linux-amd64.tar.gz && \
    mkdir -p $GOBIN $GOPATH/src/github.com/projecteru2 && \
    curl https://glide.sh/get | sh
