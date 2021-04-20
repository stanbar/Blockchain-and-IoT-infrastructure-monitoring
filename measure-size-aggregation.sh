#!/bin/bash  
while true  
do  
  export PGPASSWORD=pgpassword && \
  txns=$( psql --host localhost horizon stellar -qAt -c "SELECT count(*) FROM history_transactions txs;" )
  size=$( sudo du -s -B MB ~/docker-volumes/stellar-iot/node1/postgresql/data | awk '{print $1}' | sed 's/\([0-9]*\).*/\1/' )
  ( echo "$txns" && echo ', ' &&  echo "$size" ) | tr -d '\n' >> measure-sizes-aggregation.csv
  echo "" >> measure-sizes-aggregation.csv
  echo "txns: $txns size: $size"

  sleepTime=`echo "3600 / (1 + 200 * e(-0.0001 * ($txns - 45000270)))" | bc -l`
  echo "sleep: $sleepTime"
  sleep "$sleepTime"
done
