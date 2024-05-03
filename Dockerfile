FROM alpine:latest

COPY dist/sm2uploader-linux-amd64 /usr/local/bin/sm2uploader

ENV HOST=$HOST

ENV TIMEOUT=0.1s
ENV OCTOPRINT=:8888
EXPOSE 8888

ENTRYPOINT [ "/usr/local/bin/sm2uploader" ]
