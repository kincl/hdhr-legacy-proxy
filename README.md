# hdhr-legacy-proxy

This is a proxy that will make a HDHR legacy (HDHR Dual, HDHR 3) act like a newer version
in order to work with Plex Media Center

![diagram](design-docs/hdhr-legacy-proxy.jpg)

## Install

Compile libhdhomerun:

```
git clone https://github.com/Silicondust/libhdhomerun
cd libhdhomerun
make
```

```
go build main.go
```