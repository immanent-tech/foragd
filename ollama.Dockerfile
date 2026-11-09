FROM ollama/ollama:0.40.13@sha256:b2f0122c476bc9fe7ce679d8bb4bd555351e25e2554eefb156f61cb0a89cf429

# Start the server in the background just long enough to pull the model,
# then kill it — the weights get baked into the image layer.
RUN <<EOF
set -e
ollama serve &
until ollama list >/dev/null 2>&1; do sleep 1; done
ollama pull qwen3-embedding:0.6b
pkill ollama
EOF

ENV OLLAMA_HOST=0.0.0.0:11434
ENV OLLAMA_KEEP_ALIVE=-1
EXPOSE 11434

ENTRYPOINT ["ollama", "serve"]
