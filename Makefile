all: review

GOPATH?=`pwd`/go

export GOPATH

${GOPATH}/github.com/satori/go.uuid:
	go get github.com/satori/go.uuid

review: review.go
	go build $?

run: review
	@echo "Running on localhost:8080"
	./review

review.exe: review.go
	GOOS=windows GOARCH=amd64 go build -o $@ $?
