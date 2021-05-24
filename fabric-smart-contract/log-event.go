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
  "encoding/json"
  "fmt"
  "log"
  "time"
  "github.com/golang/protobuf/ptypes"
  "github.com/hyperledger/fabric-chaincode-go/shim"
  "github.com/hyperledger/fabric-contract-api-go/contractapi"
)

type SimpleChaincode struct {
  contractapi.Contract
}

type RequestLog struct {
  DocType        string `json:"docType"` //docType is used to distinguish the various types of objects in state database
  DeviceId string `json:"deviceId"`
  Value int `json:"value"`
  Type string `json:"type"` // HUMD, TEMP
}

type ResponseLog struct {
  ID string
  DeviceId string `json:"deviceId"`
  Value int `json:"value"`
  Type string `json:"type"` // HUMD, TEMP
}

type Aggregation struct {
  DeviceId string `json:"deviceId"`
  TimeFrame string `json:"timeFrame"` // 5sec, 30sec, 1min, 30min, 1h, 4h, 12h, 1d
  Avg int `json:"avg"`
  Max int `json:"max"`
  Min int `json:"min"`
}



func (t *SimpleChaincode) LogEvent(ctx contractapi.TransactionContextInterface, deviceId string, value int, measurementUnit string) error {
  log := &Log {
    DocType:   "log",
    DeviceId: deviceId,
    Value: valuem,
    MeasurementUnit: measurementUnit,
  }
  assetBytes, err := json.Marshal(asset)
  if err != nil {
    return err
  }
  err = ctx.GetStub().PutState(assetID, assetBytes)
  if err != nil {
    return err
  }
}

func (t *SimpleChaincode) QueryLogs(ctx contractapi.TransactionContextInterface, queryString string) ([]*ResponseLog, error) {
  return getQueryResultForQueryString(ctx, queryString)
}

func (t *SimpleChaincode) GetLogsFromDeviceId(ctx contractapi.TransactionContextInterface, deviceId string) ([]*ResponseLog, error) {
  queryString := fmt.Sprintf(`{"selector":{"docType":"log","deviceId":"%s"}}`, deviceId)
  return getQueryResultForQueryString(ctx, queryString)
}



func getQueryResultForQueryString(ctx contractapi.TransactionContextInterface, queryString string) ([]*ResponseLog, error) {
  resultsIterator, err := ctx.GetStub().GetQueryResult(queryString)
  if err != nil { 
    return nil, err
  }
  defer resultsIterator.Close()
  return constructQueryResponseFromIterator(resultsIterator)
}


// constructQueryResponseFromIterator constructs a slice of assets from the resultsIterator
func constructQueryResponseFromIterator(resultsIterator shim.StateQueryIteratorInterface) ([]*ResponseLog, error) {
  var logs []*ResponseLog
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


// InitLedger creates the initial set of assets in the ledger.
func (t *SimpleChaincode) InitLedger(ctx contractapi.TransactionContextInterface) error {
  logs := []RequestLog{
    {DocType: "log", DeviceId: "asdf", Value: 600, Type: "HUMD"},
    {DocType: "log", DeviceId: "asdf", Value: 610, Type: "HUMD"},
    {DocType: "log", DeviceId: "asdf", Value: 620, Type: "HUMD"},
    {DocType: "log", DeviceId: "asdf", Value: 630, Type: "HUMD"},
  }

  for _, log := range logs {
    err := t.LogEvent(ctx, log.DeviceId, log.Value, log.Type)
    if err != nil {
      return err 
    }
  }

  return nil
}


func main() {
  chaincode, err := contractapi.NewChaincode(&SimpleChaincode{})

  if err != nil {
    log.Panicf("Error creating logs chaincode: %v", err)
  }

  if err := chaincode.Start(); err != nil {
    log.Panicf("Error starting logs chaincode: %v", err)
  }
}

