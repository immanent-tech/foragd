FROM docker.io/alpine@sha256:51183f2cfa6320055da30872f211093f9ff1d3cf06f39a0bdb212314c5dc7375 AS builder

# Copy go from official image
COPY --from=docker.io/golang:1.26.0-alpine3.23@sha256:d4c4845f5d60c6a974c6000ce58ae079328d03ab7f721a0734277e69905473e5 /usr/local/go/ /usr/local/go/

# Install additional packages
RUN apk update && apk add sudo openssh curl git bash fish micro graphviz step-cli step-certificates nodejs npm cosign docker buildkit rootlesskit slirp4netns libcap-setcap

# Install starship
RUN cd /tmp && curl -sS https://starship.rs/install.sh | sh -s -- -y || exit -1

# Fix permissions
RUN chmod 0755 /usr/bin/newuidmap /usr/bin/newgidmap && \
    setcap cap_setuid=ep /usr/bin/newuidmap && \
    setcap cap_setgid=ep /usr/bin/newgidmap

# Add non-root user
ARG USER_NAME=vscode
ARG USER_UID=1000
ARG USER_GID=$USER_UID

RUN addgroup --gid $USER_GID $USER_NAME \
    && adduser --uid $USER_UID --ingroup $USER_NAME --shell /usr/bin/fish $USER_NAME \
    --disabled-password --gecos "" \
    && mkdir -p /etc/sudoers.d \
    && echo "$USER_NAME ALL=(ALL:ALL) NOPASSWD: ALL" > /etc/sudoers.d/$USER_NAME \
    && chmod 0440 /etc/sudoers.d/$USER_NAME

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
