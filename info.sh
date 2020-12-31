#!/bin/bash

NODE=$1
docker-compose exec ${NODE:-node1} curl "localhost:11626/info"
