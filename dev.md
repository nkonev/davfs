# tcpdump expression for capture packets
```bash
# tcpdump -A -s 0 -i lo -tttt 'tcp port 9999'
```

# testing
```bash
env GOCACHE=off go test ./...
```

```bash
go test -run TestGetCredSuccess ./cmd/davfs/ -v
```

```bash
go test ./... -cover
```

# TODO
* use postgres locks instead mutex
* cover remaining basic operations
* consider remove sqlite
* write database authentication with md5, jbcrypt
* logrus
* use postgres large object api
* sqlx
* cover remaining databases