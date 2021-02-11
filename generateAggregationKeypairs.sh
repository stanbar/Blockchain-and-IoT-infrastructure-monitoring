#!/bin/bash

for i in FIVE_SECONDS_SECRET TEN_SECONDS_SECRET FIFTEEN_SECONDS_SECRET THIRTY_SECONDS_SECRET ONE_MINUTE_SECRET FIVE_MINUTES_SECRET THIRTY_MINUTES_SECRET ONE_HOUR_SECRET SIX_HOURS_SECRET TWELVE_HOURS_SECRET ONE_DAY_SECRET;
do
  newsecret=$(docker-compose exec node1 stellar-core gen-seed | grep "Secret" | awk '{print $3}')
  echo "$i"="$newsecret"
done
