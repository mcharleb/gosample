all: review

.PHONY: env ${GOPATH}/src/github.com/satori/go.uuid

env:
	@[ "${GOPATH}" != "" ] || (echo "GOPATH not set:\n  export GOPATH=/home/go" && false)
	@[ -d  ${GOPATH}/src/github.com/satori/go.uuid ] || go get github.com/satori/go.uuid
	@[ -d  ${GOPATH}/src/github.com/xuri/excelize ] || go get github.com/xuri/excelize

/usr/bin/go:
	@echo "Missing go. Install using:"
	@echo "    sudo apt install golang"
	false

review: review.go /usr/bin/go env
	/usr/bin/go build review.go

run: review
	@echo "Running on localhost:8000"
	./review

review.exe: review.go
	GOOS=windows GOARCH=amd64 go build -o $@ $?

zip: review review.exe
	zip -r review.zip review scripts review.html data/unranked.js

clean:
	rm -f review review.exe
