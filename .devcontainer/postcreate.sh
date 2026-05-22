#!/usr/bin/bash

set -x

cd /workspace

# Update JS packages with bun.
npm clean-install || exit -1
echo 'set --export PATH "/workspace/node_modules/.bin" $PATH' >> ~/.config/fish/config.fish

# Install Go packages.
echo 'set --export PATH "$HOME/go/bin" /go/bin /usr/local/go/bin $PATH' >> ~/.config/fish/config.fish
export PATH="$HOME/go/bin:/go/bin:/usr/local/go/bin:$PATH" && \
    go mod tidy && \
    go install golang.org/x/tools/gopls@latest && \
    go install github.com/air-verse/air@latest && \
    go install github.com/a-h/templ/cmd/templ@latest && \
    go install github.com/sigstore/cosign/v3/cmd/cosign@latest && \
    go install github.com/magefile/mage@latest && \
    curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.8.0 && \
    golangci-lint custom && \
    mv /tmp/golangci-lint-v2 $(go env GOPATH)/bin/

# Install gcloud cli.
cd $HOME && \
    curl -O https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-cli-linux-x86_64.tar.gz && \
    tar -xf google-cloud-cli-linux-x86_64.tar.gz && \
    rm google-cloud-cli-linux-x86_64.tar.gz && \
    google-cloud-sdk/install.sh --usage-reporting false --quiet --additional-components app-engine-go && \
    echo 'source /home/vscode/google-cloud-sdk/path.fish.inc' >> ~/.config/fish/config.fish

# Setup docker buildx.
docker buildx create --name default-rootless --driver=docker-container --driver-opt=image=moby/buildkit:buildx-stable-1-rootless --driver-opt default-load=true \
    && docker buildx use default-rootless \
    && docker buildx inspect --bootstrap default-rootless

exit 0
