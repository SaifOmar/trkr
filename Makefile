export CGO_CFLAGS := -Wno-discarded-qualifiers

.PHONY: run build clean

run:
	go run .

build:
	go build -o trkr .

clean:
	rm -f trkr
