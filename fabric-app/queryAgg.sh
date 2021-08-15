#!/usr/bin/env bash

export FABRIC_CFG_PATH=${HOME}/go/src/github.com/stanbar/fabric-samples/config/
export CORE_PEER_LOCALMSPID="Org1MSP"
export CORE_PEER_TLS_ROOTCERT_FILE=${HOME}/go/src/github.com/stanbar/fabric-samples/test-network/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt
export CORE_PEER_MSPCONFIGPATH=${HOME}/go/src/github.com/stanbar/fabric-samples/test-network/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp
export CORE_PEER_ADDRESS=localhost:7051

/home/stanbar/go/src/github.com/stanbar/fabric-samples/bin/peer chaincode query -C mychannel -n stelliot -o localhost:7050 -c '{"Args":["GetAggregation", "1", "2021-08-10"]}'
