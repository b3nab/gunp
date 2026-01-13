

build:
	go build -o bin/gunp .

install: build
	go install .

