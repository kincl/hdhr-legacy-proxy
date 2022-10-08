# hdhr-legacy-proxy

This is a proxy that will make a HDHR legacy (HDHR Dual, HDHR 3) act like a newer version
in order to work with Plex Media Center

![diagram](design-docs/hdhr-legacy-proxy.jpg)

## Use

```
docker run -it -p 8000:8000/tcp -p 6000:6000/udp ghcr.io/kincl/hdhr-legacy-proxy:latest
```

Environment Options

| Option | Description | Default |
| ------ | ----------- | ------- |
| HDHR_LEGACY_PROXY_HOST | IP of the host running the proxy | (none) |
| HDHR_LEGACY_PROXY_PORT | Port the proxy listens on for connections from Plex | 8000 |
| HDHR_LEGACY_PROXY_TUNER_PORT | UDP port the proxy listens on for connections from the HDHR tuner | 6000 |

## TODO

- Default to using own IP for proxy host variable
- Option for manually specifying device and not doing autodiscovery
- Implement multiple tuners with a pool of resources
  - https://github.com/jackc/puddle/blob/master/pool.go
  - https://info.hdhomerun.com/info/http_api#specifying_a_tuner
- Document how to use with Docker and Kubernetes
