FROM python:3.14-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    libxml2-dev libxslt1-dev gcc g++ \
    && rm -rf /var/lib/apt/lists/*

COPY extractor/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY extractor/main.py extractor/logger.py ./

# Add labels.
LABEL org.opencontainers.image.source="https://github.com/immanent-tech/foragd"
LABEL org.opencontainers.image.url="https://foragd.app"
LABEL org.opencontainers.image.title="Foragd Content Extractor"
LABEL org.opencontainers.image.description="Extractor service for Foragd app is responsible for extractor web page content."
LABEL org.opencontainers.image.licenses="AGPL-3.0-or-later"

# Allow custom uid and gid
ARG UID=1000
ARG GID=1000

# Add user
RUN addgroup --gid "${GID}" foragd && \
    adduser --disabled-password --gecos "" --ingroup foragd \
    --uid "${UID}" foragd

RUN chown foragd:foragd /app/main.py

USER foragd

ENTRYPOINT [ "/usr/local/bin/uvicorn", "main:app" ]
