
all: build

build:
	mkdir -p bin
	go build -o bin/ ./cmd/...

build-linux:
	mkdir -p bin
	GOOS=linux GOARCH=amd64 go build -o dist/linux/amd64/ ./cmd/...

docker-build: build-linux
	docker build --platform linux/amd64 --tag hackathon-place:latest .

docker-push: docker-build
	docker tag hackathon-place:latest registry.88288338.xyz:5000/hackathon-place:latest
	docker push registry.88288338.xyz:5000/hackathon-place:latest
