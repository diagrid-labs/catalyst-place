# syntax=docker/dockerfile:1
# Deploy the application binary into a lean image
FROM gcr.io/distroless/base-debian11 AS build-release-stage

WORKDIR /

COPY dist/linux/amd64/frontend /frontend

USER nonroot:nonroot
