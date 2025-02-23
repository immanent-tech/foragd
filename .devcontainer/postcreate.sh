#!/usr/bin/bash

set -x

# Install additional packages.
sudo apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && sudo apt-get -y install --no-install-recommends micro fish ripgrep fzf pre-commit \
    && sudo apt-get -y autoremove && sudo apt-get -y clean && sudo rm -rf /var/lib/apt/lists/*

# Install starship prompt.
cd /tmp && curl -sS https://starship.rs/install.sh | sh -s -- -y || exit -1
mkdir -p ~/.config/fish
echo "starship init fish | source" >> ~/.config/fish/config.fish

cd /workspace/project

# Install parceljs.
npm install --save-dev parcel || exit -1

go mod tidy

exit 0
