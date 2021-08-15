export FABRIC_CFG_PATH=/home/stanbar/go/src/github.com/stanbar/fabric-samples/config/

peer chaincode query -C mychannel -n stelliot -c '{"Args":["GetHistoryForKey", "1"]}' | jq
