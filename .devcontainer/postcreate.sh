#!/usr/bin/bash

set -x

# Install additional packages.
sudo apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && sudo apt-get -y install --no-install-recommends micro pre-commit graphviz \
    && sudo apt-get -y autoremove && sudo apt-get -y clean && sudo rm -rf /var/lib/apt/lists/*

# Install step cli.
wget https://dl.smallstep.com/cli/docs-ca-install/latest/step-cli_amd64.deb -O deployments/step-cli_amd64.deb
sudo dpkg -i deployments/step-cli_amd64.deb

# Install starship prompt.
# cd /tmp && curl -sS https://starship.rs/install.sh | sh -s -- -y || exit -1
mkdir -p ~/.config/fish
echo "starship init fish | source" >> ~/.config/fish/config.fish

cd /workspace

# Install parceljs.
bun update || exit -1

# Install Go tools.
go install github.com/air-verse/air@latest
go install github.com/a-h/templ/cmd/templ@latest

# Clean go.mod.
go mod tidy

exit 0
