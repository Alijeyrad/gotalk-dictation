FROM archlinux:latest

ENV GOTOOLCHAIN=local
ENV GOFLAGS=-mod=mod
ENV PATH=/usr/local/go/bin:$PATH

# Install system dependencies
RUN pacman -Syu --noconfirm \
    && pacman -S --noconfirm --needed \
    wget \
    libx11 libxcursor libxrandr libxinerama libxi libxxf86vm mesa \
    alsa-utils xdotool xclip xorg-server-xvfb \
    && pacman -Scc --noconfirm

# Install Go 1.26
RUN wget -q https://go.dev/dl/go1.26.0.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go1.26.0.linux-amd64.tar.gz \
    && rm go1.26.0.linux-amd64.tar.gz

WORKDIR /workspace
