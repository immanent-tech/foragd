FROM ollama/ollama:0.40.6@sha256:265409e1d931028a8f7d5a32bcbafddd7bffbc6cc629d77efe9560adf692a5e7

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
