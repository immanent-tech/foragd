FROM ollama/ollama:0.34.2@sha256:da6e0dc5651df159e45686fd663c4dbe1624a52c44d7280eeac1551d8f865532

# Start the server in the background just long enough to pull the model,
# then kill it — the weights get baked into the image layer.
RUN <<EOF
(ollama serve &)
sleep 5
ollama pull qwen3-embedding:0.6b
pkill ollama
exit 0
EOF

ENV OLLAMA_HOST=0.0.0.0:11434
ENV OLLAMA_KEEP_ALIVE=-1
EXPOSE 11434

ENTRYPOINT ["ollama", "serve"]
