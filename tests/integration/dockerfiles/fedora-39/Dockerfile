FROM fedora:39

ENV GOTOOLCHAIN=local
ENV GOFLAGS=-mod=mod
ENV PATH=/usr/local/go/bin:$PATH

# Install system dependencies
RUN dnf install -y \
    wget \
    libX11-devel libXcursor-devel libXrandr-devel \
    libXinerama-devel libXi-devel libXxf86vm-devel mesa-libGL-devel \
    alsa-utils xdotool xclip xorg-x11-server-Xvfb \
    && dnf clean all

# Install Go 1.23
RUN wget -q https://go.dev/dl/go1.23.5.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go1.23.5.linux-amd64.tar.gz \
    && rm go1.23.5.linux-amd64.tar.gz

WORKDIR /workspace
