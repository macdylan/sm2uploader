FROM alpine:3.22

COPY dist/sm2uploader-linux-amd64 /usr/local/bin/sm2uploader

# Persist discovered printers (and their tokens) across container restarts.
# Mount a volume at /data to keep hosts.yaml.
ENV KNOWN_HOSTS=/data/hosts.yaml
VOLUME /data

ENV TIMEOUT=0.1s
ENV OCTOPRINT=:8888
EXPOSE 8888

ENTRYPOINT [ "/usr/local/bin/sm2uploader" ]
