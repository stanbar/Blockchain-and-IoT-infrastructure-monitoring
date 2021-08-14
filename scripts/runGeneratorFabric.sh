#!/usr/bin/env bash

docker run --rm --network=host --env-file generator.env -v /home/stanbar/go/src/github.com/stanbar/fabric-samples/test-network/organizations/peerOrganizations/org1.example.com/:/home/stanbar/go/src/github.com/stanbar/fabric-samples/test-network/organizations/peerOrganizations/org1.example.com/ stasbar/stellot-generator-fabric
