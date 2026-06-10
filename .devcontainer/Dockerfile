ARG CONTAINER_VERSION=dev-ubuntu26.04

FROM mcr.microsoft.com/devcontainers/base:${CONTAINER_VERSION} AS builder

ARG NODE_VERSION=v24.16.0
ARG GO_VERSION=1.26.4

# Install additional packages
RUN DEBIAN_FRONTEND=noninteractive apt -y update && \
    apt -y install fish micro graphviz ca-certificates \
    python3

# Add the Zyte CA cert.
ADD https://docs.zyte.com/_static/zyte-ca.crt /usr/local/share/ca-certificates/zyte-ca.crt
RUN update-ca-certificates

# Copy go from official image
COPY --from=docker.io/golang:${GO_VERSION} /usr/local/go/ /usr/local/go/

# Download and install node:
RUN sudo curl -L -o- https://nodejs.org/dist/${NODE_VERSION}/node-${NODE_VERSION}-linux-x64.tar.xz \
    | tar xJvf - --strip-components=1 --exclude=CHANGELOG.md --exclude=LICENSE --exclude=README.md -C /usr/local

# Download and install docker.
RUN cd /tmp && curl -fsSL https://get.docker.com -o get-docker.sh && sudo sh ./get-docker.sh && rm /tmp/get-docker.sh

# Dowload and install starship.
RUN cd /tmp && curl -sS https://starship.rs/install.sh | sh -s -- -y || exit -1

# # Add non-root user
ARG USER_NAME=vscode
ARG USER_UID=1000
ARG USER_GID=$USER_UID

RUN echo $USER_UID:100000:65536 > /etc/subuid
RUN echo $USER_GID:100000:65536 > /etc/subgid

USER $USER_NAME

ENV _CONTAINERS_USERNS_CONFIGURED=""

# The base container sets XDG_CACHE_HOME XDG_CONFIG_HOME specifically for the root user, we can't unset them in a way that vscode will pick up, so we set them to values for the new user.
# Installing go extensions via vscode use these paths so if we just leave it set to /root/.cache we'll get permission errors.
ENV XDG_CONFIG_HOME=/home/$USER_NAME/.config
ENV XDG_CACHE_HOME=/home/$USER_NAME/.cache
# Set up XDG_RUNTIME_DIR
RUN export XDG_RUNTIME_DIR=/tmp/$USER_UID-runtime-dir \
    && mkdir $XDG_RUNTIME_DIR \
    && chmod 0700 $XDG_RUNTIME_DIR
ENV XDG_RUNTIME_DIR=/tmp/$USER_UID-runtime-dir
# Set up shell(s).
RUN mkdir -p ~/.config/fish \
    && echo "starship init fish | source" >>~/.config/fish/config.fish \
    && echo 'eval "$(starship init bash)"' >>~/.bashrc

ENTRYPOINT ["/usr/bin/fish"]
