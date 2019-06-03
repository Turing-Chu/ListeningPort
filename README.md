# A little Project

> get process information which listing port, then store it mysql.
> After that, show in browser by grafana.

## Init
```bash
export GO111MODULE=on
go mod init 
go mod tidy
go mod vendor
```

## Build

```bash
# build on Mac, because of can not site golang.org on Alibaba ECS
# CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o listening_port main.go
go build 
```

## Add to crontab

```text
*/5 * * * * ./listening_port -c config.yml

```
