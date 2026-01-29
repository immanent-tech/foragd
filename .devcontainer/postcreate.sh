#!/usr/bin/bash

set -x

# Add starship to fish shell.
mkdir -p ~/.config/fish
echo "starship init fish | source" >>~/.config/fish/config.fish

# Add starship to bash shell.
echo 'eval "$(starship init bash)"' >>~/.bashrc

cd /workspace

# Update JS packages with bun.
npm update || exit -1

# Install Go packages.
go mod tidy
go install github.com/air-verse/air@latest
go install github.com/a-h/templ/cmd/templ@latest
go install golang.org/x/tools/gopls@latest
curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.8.0
cd /tmp && \
    golangci-lint custom && \
    mv golangci-lint $(go env GOPATH)/bin/golangci-lint-v2

# Set up fish shell.
mkdir -p ~/.config/fish
echo 'set --export PATH "/workspace/node_modules/.bin" $PATH' >> ~/.config/fish/config.fish
echo 'set --export PATH "$HOME/go/bin" /go/bin /usr/local/go/bin $PATH' >> ~/.config/fish/config.fish

# Install Stripe CLI.
cd /tmp \
    && curl -L -O https://github.com/stripe/stripe-cli/releases/download/v1.33.2/stripe_1.33.2_linux_x86_64.tar.gz \
    && tar xvf stripe* \
    && sudo mv stripe /usr/local/bin

exit 0
