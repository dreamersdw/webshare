run: build
	./webshare

build:
	go-bindata-assetfs  static/...
	go build

