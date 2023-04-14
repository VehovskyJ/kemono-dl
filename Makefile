.PHONY: all clean

BUILD_PATH=build
BINARY=kemono-dl

all: linux windows arm mac

clean:
	rm -rf $(BUILD_PATH)

linux:
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_PATH)/$(BINARY)_linux_amd64

windows:
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_PATH)/$(BINARY)_windows_amd64

arm:
	GOOS=linux GOARCH=arm GOARM=7 go build -o $(BUILD_PATH)/$(BINARY)_linux_arm7

mac:
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_PATH)/$(BINARY)_darwin_amd64
