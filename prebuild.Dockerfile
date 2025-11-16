FROM docker.io/almalinux:10-kitten-20250909

RUN dnf update -y \
    && dnf install -y \
        epel-release \
        libvirt \
        libvirt-devel \
        gcc \
        tar

RUN curl -LO https://golang.org/dl/go1.25.4.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.25.4.linux-amd64.tar.gz && \
    rm go1.25.4.linux-amd64.tar.gz

ENV GOROOT=/usr/local/go
ENV PATH=$PATH:$GOROOT/bin

RUN mkdir -p /app/go/bin && chown -R $USER:$USER /app
WORKDIR /app

RUN ["/bin/bash", "-c", "\
    echo 'export GOPATH=/app/go' >> /root/.bashrc && \
    echo 'export PATH=$PATH:$GOPATH/bin' >> /root/.bashrc \
"]

