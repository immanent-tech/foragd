FROM docker.io/alpine AS builder

# Copy go from official image
COPY --from=golang:1.25.0-alpine /usr/local/go/ /usr/local/go/

# Install additional packages
RUN apk update && apk add bash fish micro graphviz curl step-cli step-certificates git sudo nodejs npm

# Install golangci-lint
RUN version=2.1.6 \
    && curl -fsSL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /go/bin v$version \
    && /go/bin/golangci-lint version

# Install starship
RUN cd /tmp && curl -sS https://starship.rs/install.sh | sh -s -- -y || exit -1

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

USER $USER_NAME

# Install pulumictl
RUN curl -fsSL https://get.pulumi.com | sh

# Install bun
RUN curl -fsSL https://bun.com/install | bash

RUN <<EOF
    mkdir -p ~/.config/fish
    echo 'set --export BUN_INSTALL "$HOME/.bun"' >> ~/.config/fish/config.fish
    echo 'set --export PATH $BUN_INSTALL/bin $PATH' >> ~/.config/fish/config.fish
    echo 'set --export PULUMI_INSTALL "$HOME/.pulumi"' >> ~/.config/fish/config.fish
    echo 'set --export PATH $PULUMI_INSTALL/bin $PATH' >> ~/.config/fish/config.fish
    echo 'set --export PATH "$HOME/go/bin" /go/bin /usr/local/go/bin $PATH' >> ~/.config/fish/config.fish
EOF

# The base container sets XDG_CACHE_HOME XDG_CONFIG_HOME specifically for the root user, we can't unset them in a way that vscode will pick up, so we set them to values for the new user.
# Installing go extensions via vscode use these paths so if we just leave it set to /root/.cache we'll get permission errors.
ENV XDG_CONFIG_HOME=/home/$USER_NAME/.config
ENV XDG_CACHE_HOME=/home/$USER_NAME/.cache

ENTRYPOINT ["/usr/bin/fish"]
