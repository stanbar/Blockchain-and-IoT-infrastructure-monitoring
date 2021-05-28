/*
Chaincode executions

===== Log event
peer chaincode invoke -C myc1 -n log_event -c '{"Args":["LogEvent","device-id",600,"HUMD","2021-05-25T11:40:52.280Z"]}'

===== Invoke Aggregation

peer chaincode invoke -C myc1 -n log_event -c '{"Args":["AggregateInterval", "2021-05-25T11:40:52.280Z"]}'

===== Query count predicate

1. GET ALL TRANSACTIONS WHERE $time$ OR $time$ OR $time$

peer chaincode invoke -C myc1 -n log_event -c '{"Args":["GetFirst", "device-id", "2021-05-25", "2021-05-24", "2021-05-23"]}'

2. GET ALL TRANSACTIONS WHERE $value_{min}$ $<$ $sensor_n.value$ $<$ $value_{max}$

peer chaincode invoke -C myc1 -n log_event -c '{"Args":["GetSecond", "device-id", 600, 700]}'

3. GET AVG($sensor_n$, $unit$) WHERE $created\_at$ $>$ time AND $created\_at$ $<$ $time$

peer chaincode invoke -C myc1 -n log_event -c '{"Args":["GetThird", "device-id", "2021-05-25", "2021-05-24"]}'

4. GET COUNT(*) WHERE SENSOR = $sensor_n$ AND UNIT=$unit$ AND $created\_at$ $>$ $time$

peer chaincode invoke -C myc1 -n log_event -c '{"Args":["GetFourth", "device-id", "2021-05-25"]}'

*/

package main

import (
	"log"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
	"github.com/stellot/stellot-iot/fabric-smart-contract/chaincode"
)

func main() {
	logsChaincode, err := contractapi.NewChaincode(&chaincode.SmartContract{})

	if err != nil {
		log.Panicf("Error creating logs chaincode: %v", err)
	}

	if err := logsChaincode.Start(); err != nil {
		log.Panicf("Error starting logs chaincode: %v", err)
	}
}
