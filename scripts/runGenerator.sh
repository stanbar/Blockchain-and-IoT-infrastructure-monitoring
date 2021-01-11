#!/usr/bin/env bash

docker run -d --net stellot-iot_default --env-file generator.env stasbar/stellot-generator
