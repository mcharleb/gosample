all: test

test: test.go
	go build test.go

run: test
	@echo "Running on localhost:8080"
	./test
