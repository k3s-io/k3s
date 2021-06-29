# Docker image for building the Go app
FROM golang:latest

WORKDIR /app
ADD . /app

RUN GO111MODULE=on go mod download
