peer chaincode query -C mychannel -n logs5 -c '{"Args":["GetHistoryForKey", "asdf"]}' | jq
