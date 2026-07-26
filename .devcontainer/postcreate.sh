#!/usr/bin/bash

set -x

cd /workspace

# Oh My Posh.
source base/.devcontainer/postcreate.scripts.d/oh-my-posh.sh

# Zyte.
source base/.devcontainer/postcreate.scripts.d/zyte.sh

# Google Cloud.
source base/.devcontainer/postcreate.scripts.d/gcloud.sh

# Frontend.
source base/.devcontainer/postcreate.scripts.d/frontend.sh

# Go.
source base/.devcontainer/postcreate.scripts.d/go.sh
