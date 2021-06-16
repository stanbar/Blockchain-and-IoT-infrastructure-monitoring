#!/usr/bin/env bash

export $(grep -v '^#' generator.env | xargs -d '\n')
