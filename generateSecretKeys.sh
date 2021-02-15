#!/bin/bash

output=""
for i in {1..50}
do
  newsecret=$(docker-compose exec node1 stellar-core gen-seed | grep "Secret" | awk '{print $3}')
  echo $newsecret
  echo "$newsecret" >> secrets.txt
done

