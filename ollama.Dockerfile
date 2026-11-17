FROM ollama/ollama:0.40.15@sha256:0e59999e0e8f9fd4e704bd788a89b7e4cad399367fb1365f8c102ac662288761

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
