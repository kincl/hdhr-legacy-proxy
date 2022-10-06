FROM docker.io/library/python:latest as libbuild

WORKDIR /tmp
RUN git clone https://github.com/kincl/libhdhomerun.git && \
    cd libhdhomerun && \
    make

FROM docker.io/library/golang:1.19.2 as build

COPY . /hdhr-legacy-proxy
WORKDIR /hdhr-legacy-proxy

COPY --from=libbuild /tmp/libhdhomerun /hdhr-legacy-proxy/libhdhomerun
RUN go build -o hdhr-legacy-proxy main.go

FROM docker.io/redhat/ubi8-minimal:latest

COPY --from=libbuild /tmp/libhdhomerun/libhdhomerun.so /usr/lib64
COPY --from=build /hdhr-legacy-proxy/hdhr-legacy-proxy /bin/hdhr-legacy-proxy

CMD ["hdhr-legacy-proxy"]
