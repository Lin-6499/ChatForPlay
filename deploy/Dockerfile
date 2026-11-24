FROM scratch
COPY dist/server/linchat-linux-amd64 /linchat
COPY --chown=10001:10001 deploy/volume/ /data/
USER 10001:10001
EXPOSE 8787
ENTRYPOINT ["/linchat"]
CMD ["-listen", "0.0.0.0:8787", "-data", "/data", "-url", "https://chat.linblog.site"]
