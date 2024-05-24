
build:
	mkdir -p bin
	go build -o bin/ ./cmd/...

docker-build:
	docker build -t hackathon-place .

docker-push:
	docker tag hackathon-place:latest public.ecr.aws/d3f9w4q8/hackathon-place:latest
	docker push public.ecr.aws/d3f9w4q8/hackathon-place:latest
