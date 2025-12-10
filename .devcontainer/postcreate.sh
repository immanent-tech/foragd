#!/usr/bin/bash

set -x

# Add starship to fish shell.
mkdir -p ~/.config/fish
echo "starship init fish | source" >>~/.config/fish/config.fish

# Add starship to bash shell.
echo 'eval "$(starship init bash)"' >>~/.bashrc

cd /workspace

# Update JS packages with bun.
bun update || exit -1

# Install Go packages.
go mod tidy
go install github.com/air-verse/air@latest
go install github.com/a-h/templ/cmd/templ@latest

# Install Stripe CLI.
curl https://github.com/stripe/stripe-cli/releases/download/v1.33.0/stripe_1.33.0_linux_x86_64.tar.gz | tar xvf -C /tmp && \
    sudo mv stripe /usr/local/bin

exit 0
