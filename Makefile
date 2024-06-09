build:
	go build -o cli/$(shell basename $(PWD)) cli/main.go

tidy:
	cd cli; go mod tidy