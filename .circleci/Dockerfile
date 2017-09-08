FROM centos as prepare

MAINTAINER CMGS <ilskdw@gmail.com>
# CentOS 7.3 目前没有 http-parser 官方 pkg，另外 libgit2 的版本是 0.25.1，因此需要重新打包
RUN yum -y update && yum -y install wget rpm-build cmake libcurl-devel libssh2-devel openssl-devel zlib-devel epel-release
RUN yum -y install gyp
RUN yum -y groupinstall "Development Tools"
ENV LIBGIT2VERSION 0.25.1
ENV RELEASE 3
ENV FEDORA 26
ENV PARSER 2.7.1
RUN wget http://dl.fedoraproject.org/pub/fedora/linux/releases/$FEDORA/Everything/source/tree/Packages/l/libgit2-$LIBGIT2VERSION-$RELEASE.fc$FEDORA.src.rpm
RUN wget http://springdale.math.ias.edu/data/puias/unsupported/7/SRPMS/http-parser-$PARSER-$RELEASE.sdl7.src.rpm
RUN rpm -i libgit2-$LIBGIT2VERSION-$RELEASE.fc$FEDORA.src.rpm http-parser-$PARSER-$RELEASE.sdl7.src.rpm
WORKDIR /root/rpmbuild/SPECS
RUN rpmbuild -ba http-parser.spec && rpm -i ../RPMS/x86_64/http-parser-$PARSER-$RELEASE.el7.centos.x86_64.rpm ../RPMS/x86_64/http-parser-devel-$PARSER-$RELEASE.el7.centos.x86_64.rpm
RUN rpmbuild -ba --nocheck libgit2.spec
RUN cp ../RPMS/x86_64/http-parser-$PARSER-$RELEASE.el7.centos.x86_64.rpm ../RPMS/x86_64/http-parser-devel-$PARSER-$RELEASE.el7.centos.x86_64.rpm /tmp
RUN cp ../RPMS/x86_64/libgit2-$LIBGIT2VERSION-$RELEASE.el7.centos.x86_64.rpm ../RPMS/x86_64/libgit2-devel-$LIBGIT2VERSION-$RELEASE.el7.centos.x86_64.rpm /tmp

FROM centos

MAINTAINER CMGS <ilskdw@gmail.com>

ENV LIBGIT2VERSION 0.25.1
ENV RELEASE 3
ENV FEDORA 26
ENV PARSER 2.7.1
COPY --from=prepare /tmp/libgit2-$LIBGIT2VERSION-$RELEASE.el7.centos.x86_64.rpm /tmp
COPY --from=prepare /tmp/libgit2-devel-$LIBGIT2VERSION-$RELEASE.el7.centos.x86_64.rpm /tmp
COPY --from=prepare /tmp/http-parser-$PARSER-$RELEASE.el7.centos.x86_64.rpm /tmp
COPY --from=prepare /tmp/http-parser-devel-$PARSER-$RELEASE.el7.centos.x86_64.rpm /tmp
RUN yum -y update && \
    yum -y groupinstall "Development Tools" && \
    yum -y install NetworkManager epel-release sudo which wget ruby rubygems ruby-devel openssl-devel zlib-devel && \
    rpm -i /tmp/*.rpm && rm -rf /tmp/*.rpm && \
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
