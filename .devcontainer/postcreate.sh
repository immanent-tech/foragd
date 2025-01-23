#!/usr/bin/bash

set -x

# Install starship prompt.
cd /tmp && curl -sS https://starship.rs/install.sh | sh -s -- -y || exit -1
mkdir -p ~/.config/fish
# echo "starship init fish | source" >> ~/.config/fish/config.fish

cd /workspace/project

# Install parceljs.
npm install --save-dev parcel || exit -1

go mod tidy

# Install latest templ command.
go install github.com/a-h/templ/cmd/templ@latest
# Install latest air command.
go install github.com/air-verse/air@latest

exit 0
