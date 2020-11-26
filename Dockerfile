ARG ARCH="amd64"
ARG OS="linux"
FROM quay.io/prometheus/busybox-${OS}-${ARCH}:latest

LABEL maintainer="Jennings Liu <jenningsloy318@gmail.com>"

COPY LICENSE /LICENSE
COPY netapp_exporter /bin/netapp_exporter

USER nobody
ENTRYPOINT ["/bin/netapp_exporter"]
EXPOSE 9609
