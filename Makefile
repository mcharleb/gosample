all: test

test: test.go
	go build $?

run: test
	@echo "Running on localhost:8080"
	./test

test.exe: test.go
	GOOS=windows GOARCH=amd64 go build -o $@ $?
