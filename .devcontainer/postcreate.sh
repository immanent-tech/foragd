#!/usr/bin/bash

set -x

sudo chown -R vscode:vscode /workspace/node_modules \
    /home/vscode/go \
    /home/vscode/.cache \
    /workspace/tmp
