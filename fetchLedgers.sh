#!/bin/bash

NO_LEDGERS=$1
curl -s "localhost:9001/ledgers?order=desc&limit=${NO_LEDGERS:-10}" | jq '[._embedded.records[] | {successful_transaction_count}[]]'
