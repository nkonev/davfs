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