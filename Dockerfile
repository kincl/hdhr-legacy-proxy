FROM registry.access.redhat.com/ubi8/go-toolset:1.19 as build

WORKDIR /tmp
RUN git clone https://github.com/kincl/libhdhomerun.git && \
    cd libhdhomerun && \
    make

COPY --chown=1001:0 . /hdhr-legacy-proxy
WORKDIR /hdhr-legacy-proxy

RUN cp -r /tmp/libhdhomerun /hdhr-legacy-proxy/libhdhomerun
RUN go build -o hdhr-legacy-proxy cmd/root.go

FROM docker.io/redhat/ubi8-minimal:latest

COPY --from=build /tmp/libhdhomerun/libhdhomerun.so /usr/lib64
COPY --from=build /hdhr-legacy-proxy/hdhr-legacy-proxy /bin/hdhr-legacy-proxy

CMD ["hdhr-legacy-proxy"]
