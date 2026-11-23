FROM ollama/ollama:0.40.16@sha256:910d0d8107b538fb0894e63163eb195de0cc7aefcbc666dc62d3b18f71124592

# Start the server in the background just long enough to pull the model,
# then kill it — the weights get baked into the image layer.
RUN <<EOF
set -e
ollama serve &
until ollama list >/dev/null 2>&1; do sleep 1; done
ollama pull qwen3-embedding:4b
pkill ollama
EOF

ENV OLLAMA_HOST=0.0.0.0:11434
ENV OLLAMA_KEEP_ALIVE=-1
ENV OLLAMA_NUM_PARALLEL=8
EXPOSE 11434

ENTRYPOINT ["ollama", "serve"]
