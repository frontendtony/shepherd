.PHONY: build test run lint clean

build:
	go build -o shepherd .

test:
	go test -race ./...

run: build
	./shepherd

lint:
	go vet ./...

clean:
	rm -f shepherd
	go clean
