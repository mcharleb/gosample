all: review

review: review.go
	go build $?

package: review
	
	
run: review
	@echo "Running on localhost:8080"
	./review

review.exe: review.go
	GOOS=windows GOARCH=amd64 go build -o $@ $?
