BIN_DIR := ./bin

.PHONY: run run-dev build build-windows deploy-windows clean

run:
	go run .

run-dev:
	go run -tags dev .

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/trkr .

build-windows:
	mkdir -p $(BIN_DIR)
	GOOS=windows GOARCH=amd64 go build -o $(BIN_DIR)/trkr.exe .

deploy-windows: build-windows
	cp $(BIN_DIR)/trkr.exe ~/Windows/

clean:
	rm -rf $(BIN_DIR)
