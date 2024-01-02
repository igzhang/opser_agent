build:
	go build -o build/opserc main.go

clean:
	rm -rf build

build-win:
	GOOS=windows GOARCH=amd64 go build -o build/opserc.exe main.go
