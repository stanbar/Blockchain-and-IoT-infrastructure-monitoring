#!/bin/bash

REQ=$1
LIMIT=$2
SORT=$2
curl "localhost:9001/${REQ}?limit=${LIMIT:-5}&sort=${SORT:-asc}"
echo "${REQ} ${SORT:-5}"
