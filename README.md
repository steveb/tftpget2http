# tftpget2http

This is a TFTP to HTTP read-only proxy server. It is intended to be run as a container so:

* Environment variables are used to configure behaviour
* A `Dockerfile` is included to build an image

To build and run as a local binary:

```
# go build -o tftpget2http main.go
# HTTP_URL=http://localhost:8080/tftpboot/ LISTEN=:6969 ./tftpget2http
```

To build and run as container using the host network:
```
# buildah bud -f ./Dockerfile --format docker --tls-verify=true -t tftpget2http ./
# podman run --net=host --rm -e HTTP_URL=http://localhost:8080/tftpboot/ -e LISTEN=:6969 localhost/tftpget2http:latest
```

Running without any variables will print usage:
```
./tftpget2http
LISTEN=:69
HTTP_MAX_IDLE=10
HTTP_TIMEOUT=1
TFTP_TIMEOUT=1
TFTP_RETRIES=5
Error parsing environment variables:
HTTP_URL: mandatory value missing

Usage:

HTTP_MAX_IDLE: Maximum idle connections for HTTP requests (Default 10)

HTTP_TIMEOUT: HTTP timeout limit in seconds (Default 1)

TFTP_TIMEOUT: TFTP timeout limit in seconds (Default 1)

TFTP_RETRIES: TFTP retries (Default 5)

HTTP_URL: Address and port to listen to (Mandatory)

LISTEN: Address and port to listen to (Default :69)

```