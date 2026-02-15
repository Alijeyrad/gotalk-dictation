FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive
ENV GOTOOLCHAIN=local
ENV GOFLAGS=-mod=mod
ENV PATH=/usr/local/go/bin:$PATH

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    wget ca-certificates \
    libx11-dev libxcursor-dev libxrandr-dev libxinerama-dev \
    libxi-dev libxxf86vm-dev libgl1-mesa-dev \
    alsa-utils xdotool xclip xvfb \
    && rm -rf /var/lib/apt/lists/*

# Install Go 1.23
RUN wget -q https://go.dev/dl/go1.24.2.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go1.24.2.linux-amd64.tar.gz \
    && rm go1.24.2.linux-amd64.tar.gz

WORKDIR /workspace
