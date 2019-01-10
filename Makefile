BINARY_NAME=kubernetes-image-puller
DOCKERIMAGE_NAME=kubernetes-image-puller
DOCKERIMAGE_TAG=latest

all: build docker

build:
	GOOS=linux go build -v -o ./bin/${BINARY_NAME} ./cmd/main.go

docker:
	docker build -t ${DOCKERIMAGE_NAME}:${DOCKERIMAGE_TAG} -f ./docker/Dockerfile .

clean:
	rm -rf ./bin