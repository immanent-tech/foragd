FROM ollama/ollama:0.32.4@sha256:0ab10b9b9dc5f50d30dc61aec25e3316822ca22cf0f27d4e98d74cc7dedd7c80

# Start the server in the background just long enough to pull the model,
# then kill it — the weights get baked into the image layer.
RUN <<EOF
(ollama serve &)
ollama pull qwen3-embedding:0.6b
pkill ollama
exit 0
EOF

ENV OLLAMA_HOST=0.0.0.0:11434
ENV OLLAMA_KEEP_ALIVE=-1
EXPOSE 11434

ENTRYPOINT ["ollama", "serve"]
