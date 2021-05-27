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
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/hyperledger/fabric-chaincode-go/shim"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
	pb "github.com/hyperledger/fabric-protos-go/peer"
)

type SmartContract struct {
	contractapi.Contract
}

type Log struct {
	ID              string `json:"ID"`
	DeviceId        string `json:"deviceId"`
	Value           int    `json:"value"`
	MeasurementUnit string `json:"measurementUnit"` // HUMD, TEMP
}

type Aggregation struct {
	DeviceId  string `json:"deviceId"`
	TimeFrame string `json:"timeFrame"` // 5sec, 30sec, 1min, 30min, 1h, 4h, 12h, 1d
	Avg       int    `json:"avg"`
	Max       int    `json:"max"`
	Min       int    `json:"min"`
}

// InitLedger creates the initial set of assets in the ledger.
func (t *SmartContract) InitLedger(ctx contractapi.TransactionContextInterface) error {
	logs := []Log{
		{ID: "asdf-2021-05-25", DeviceId: "asdf", Value: 600, MeasurementUnit: "HUMD"},
		{ID: "asdf-2021-05-25", DeviceId: "asdf", Value: 610, MeasurementUnit: "HUMD"},
		{ID: "asdf-2021-05-25", DeviceId: "asdf", Value: 620, MeasurementUnit: "HUMD"},
		{ID: "asdf-2021-05-25", DeviceId: "asdf", Value: 630, MeasurementUnit: "HUMD"},

		{ID: "fdsa-2021-05-25", DeviceId: "fdsa", Value: 170, MeasurementUnit: "TEMP"},
		{ID: "fdsa-2021-05-25", DeviceId: "fdsa", Value: 180, MeasurementUnit: "TEMP"},
		{ID: "fdsa-2021-05-25", DeviceId: "fdsa", Value: 190, MeasurementUnit: "TEMP"},
	}

	for _, log := range logs {
		logJSON, err := json.Marshal(log)
		if err != nil {
			return err
		}
		err = ctx.GetStub().PutState(log.ID, logJSON)
		if err != nil {
			return fmt.Errorf("failed to put to world state. %v", err)
		}
	}

	return nil
}

func (t *SmartContract) RecordLog(ctx contractapi.TransactionContextInterface, id string, deviceId string, value int, measurementUnit string) error {
	log := &Log{
		ID:              id,
		DeviceId:        deviceId,
		Value:           value,
		MeasurementUnit: measurementUnit,
	}
	assetBytes, err := json.Marshal(log)
	if err != nil {
		return err
	}
	err = ctx.GetStub().PutState(id, assetBytes)
	if err != nil {
		return err
	}
	return nil
}

func (t *SmartContract) GetHistoryForKey(ctx contractapi.TransactionContextInterface, id string) pb.Response {
	resultsIterator, err := ctx.GetStub().GetHistoryForKey(id)
	if err != nil {
		return shim.Error(err.Error())
	}
	defer resultsIterator.Close()

	// buffer is a JSON array containing historic values for the marble
	var buffer bytes.Buffer
	buffer.WriteString("[")

	bArrayMemberAlreadyWritten := false
	for resultsIterator.HasNext() {
		response, err := resultsIterator.Next()
		if err != nil {
			return shim.Error(err.Error())
		}
		// Add a comma before array members, suppress it for the first array member
		if bArrayMemberAlreadyWritten == true {
			buffer.WriteString(",")
		}
		buffer.WriteString("{\"TxId\":")
		buffer.WriteString("\"")
		buffer.WriteString(response.TxId)
		buffer.WriteString("\"")

		buffer.WriteString(", \"Value\":")
		// if it was a delete operation on given key, then we need to set the
		//corresponding value null. Else, we will write the response.Value
		//as-is (as the Value itself a JSON marble)
		if response.IsDelete {
			buffer.WriteString("null")
		} else {
			buffer.WriteString(string(response.Value))
		}

		buffer.WriteString(", \"Timestamp\":")
		buffer.WriteString("\"")
		buffer.WriteString(time.Unix(response.Timestamp.Seconds, int64(response.Timestamp.Nanos)).String())
		buffer.WriteString("\"")

		buffer.WriteString(", \"IsDelete\":")
		buffer.WriteString("\"")
		buffer.WriteString(strconv.FormatBool(response.IsDelete))
		buffer.WriteString("\"")

		buffer.WriteString("}")
		bArrayMemberAlreadyWritten = true
	}
	buffer.WriteString("]")

	fmt.Printf("- getHistoryForMarble returning:\n%s\n", buffer.String())

	return shim.Success(buffer.Bytes())
}

func (t *SmartContract) QueryLogs(ctx contractapi.TransactionContextInterface, queryString string) ([]*Log, error) {
	return getQueryResultForQueryString(ctx, queryString)
}

func (t *SmartContract) GetLogsFromDeviceId(ctx contractapi.TransactionContextInterface, deviceId string) ([]*Log, error) {
	queryString := fmt.Sprintf(`{"selector":{"docType":"log","deviceId":"%s"}}`, deviceId)
	return getQueryResultForQueryString(ctx, queryString)
}

func getQueryResultForQueryString(ctx contractapi.TransactionContextInterface, queryString string) ([]*Log, error) {
	resultsIterator, err := ctx.GetStub().GetQueryResult(queryString)
	if err != nil {
		return nil, err
	}
	defer resultsIterator.Close()
	return constructQueryResponseFromIterator(resultsIterator)
}

// constructQueryResponseFromIterator constructs a slice of assets from the resultsIterator
func constructQueryResponseFromIterator(resultsIterator shim.StateQueryIteratorInterface) ([]*Log, error) {
	var logs []*Log
	for resultsIterator.HasNext() {

		queryResult, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}
		var log Log
		err = json.Unmarshal(queryResult.Value, &log)

		if err != nil {
			return nil, err
		}

		logs = append(logs, &log)

	}
	return logs, nil
}

func main() {
	chaincode, err := contractapi.NewChaincode(&SmartContract{})

	if err != nil {
		log.Panicf("Error creating logs chaincode: %v", err)
	}

	if err := chaincode.Start(); err != nil {
		log.Panicf("Error starting logs chaincode: %v", err)
	}
}
