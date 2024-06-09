build:
	go build -o $(shell basename $(PWD)) cli/*.go

tidy:
	cd cli; go mod tidy

run:
	go run cli/*.go $(ARGS)