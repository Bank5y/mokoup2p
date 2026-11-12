# makensis for building the Windows installer on any host:
#   docker build -t mokoup2p-nsis packaging/windows
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends nsis && rm -rf /var/lib/apt/lists/*
WORKDIR /work
ENTRYPOINT ["makensis"]
