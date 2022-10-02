# hdhr-legacy-proxy

This is a proxy that will make a HDHR legacy (HDHR Dual, HDHR 3) act like a newer version
in order to work with Plex Media Center

## Install

Compile libhdhomerun:

```
git clone https://github.com/Silicondust/libhdhomerun
cd libhdhomerun
make
```

Download this package and deps

```
pip install -r requirements.txt
pip install git+https://github.com/kincl/hdhr.git
```
