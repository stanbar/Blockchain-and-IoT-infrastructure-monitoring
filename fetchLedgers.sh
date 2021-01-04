#!/bin/bash

NO_LEDGERS=$1
VERBOSE=$2
echo "${VERBOSE}"
if [ -z "${VERBOSE}" ]
then
  QUERY="[._embedded.records[] | {sequence, successful_transaction_count}[]]"
else
  QUERY="[._embedded.records[] | {successful_transaction_count, failed_transaction_count}]"
fi
curl -s "localhost:9001/ledgers?order=desc&limit=${NO_LEDGERS:-10}" | jq "${QUERY}"
