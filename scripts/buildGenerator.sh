#!/usr/bin/env bash

docker build -t stasbar/stellot-generator --file ./cmd/generator/Dockerfile . && \
docker push stasbar/stellot-generator
