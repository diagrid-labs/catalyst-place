
all: build

build:
	mkdir -p bin
	go build -o bin/ ./cmd/...

docker-build: build
	docker build --platform linux/amd64 --tag hackathon-place:latest .

docker-push: docker-build
	docker tag hackathon-place:latest registry.88288338.xyz:5000/hackathon-place:latest
	docker push registry.88288338.xyz:5000/hackathon-place:latest
