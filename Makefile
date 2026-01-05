.PHONY: build

install:
	go install ./cmd/twig

build:
	go build -o out/twig ./cmd/twig
