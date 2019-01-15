# davfs

WebDAV filesystem

## Building
```
rm -rf build
go build -o build/davfs ./cmd/davfs/davfs.go
```

## Usage

```
$ davfs
```

## Supported Drivers

|Driver    |Options to be specified           |
|----------|----------------------------------|
|file      |-driver=file -source=/path/to/root|
|memory    |-driver=memory                    |
|sqlite3   |-driver=sqlite3 -source=fs.db     |
|mysql     |-driver=mysql -source=blah...     |
|postgresql|-driver=postgres -source=blah...  |


## Installation

```
$ go get github.com/nkonev/davfs/cmd/davfs
```

At the first time, you need to create filesystem for database drivers you specified like below.

```
$ davfs -driver=sqlite3 -source=fs.db -create
```

# In-memory example

```
$ davfs -driver=memory -cred=user:password
```

## License

MIT

## Author

Yasuhiro Matsumoto (a.k.a mattn)
