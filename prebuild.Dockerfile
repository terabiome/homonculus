FROM docker.io/almalinux:10-kitten-20250909

RUN dnf update -y \
    && dnf install -y \
        epel-release \
        libvirt \
        tar

RUN curl -LO https://golang.org/dl/go1.25.4.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.25.4.linux-amd64.tar.gz && \
    rm go1.25.4.linux-amd64.tar.gz

RUN mkdir /app && chown -R $USER:$USER /app
WORKDIR /app

RUN ["/bin/bash", "-c", "echo 'export PATH=$PATH:/usr/local/go/bin' >> /root/.bashrc"]
