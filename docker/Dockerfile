# MeowX Docker 版：控制服务 + 内核 + 官方 tailscale（静态程序，由 scripts/build-rootfs.sh 备好在 rootfs/）。
# alpine 只为带上 CA 证书、时区和 iptables-legacy（喵窝在没有 nf_tables 的内核 / 用 legacy 的 Docker 上要用）。
FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata iptables iptables-legacy
COPY rootfs/ /
ENV PATH=/usr/bin:/usr/sbin:/bin:/sbin \
    MEOWX_DATA=/data \
    MEOWX_UI_PORT=7899
VOLUME /data
EXPOSE 7890 7899
STOPSIGNAL SIGTERM
ENTRYPOINT ["/usr/bin/meowx-agent"]
