#!/bin/bash  
while true  
do  
  export PGPASSWORD=pgpassword && \
    psql --host localhost horizon stellar -c "SELECT count(*) FROM history_transactions txs;" && \
    sudo du -s -B MB ~/docker-volumes/stellar-iot/node1/postgresql/data
      sleep 300  
done
