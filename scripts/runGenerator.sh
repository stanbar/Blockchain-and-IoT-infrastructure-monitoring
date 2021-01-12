#!/usr/bin/env bash

docker run --rm --net stellot-iot_default --env-file generator.env stasbar/stellot-generator
