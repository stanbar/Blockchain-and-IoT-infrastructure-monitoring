#!/usr/bin/env bash

docker run --net stellot-iot_default --env-file generator.env stasbar/stellot-sendtx
