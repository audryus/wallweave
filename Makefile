build:
	go build -o bin/wallweave .

run: build
	qs -p ui