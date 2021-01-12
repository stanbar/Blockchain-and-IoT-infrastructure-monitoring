#!/usr/bin/env bash

docker build -t stasbar/stellot-aggregator --file ./cmd/aggregator/Dockerfile . && \
docker push stasbar/stellot-aggregator 
