.PHONY: build

install:
	go install ./cmd/gwt

build:
	go build -o out/gwt ./cmd/gwt
