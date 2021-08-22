package chaincode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hyperledger/fabric-chaincode-go/shim"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
	"github.com/hyperledger/fabric-protos-go/ledger/queryresult"
)

type SmartContract struct {
	contractapi.Contract
}

type Log struct {
	SensorID        string `json:"sensorId"`
	CreationTime    string `json:"creationTime"`
	Value           int    `json:"value"`
	MeasurementUnit string `json:"measurementUnit"` // HUMD, TEMP
}

type Aggregation struct {
	SensorID  string `json:"sensorId"`
	TimeFrame string `json:"timeFrame"` // 5sec, 30sec, 1min, 30min, 1h, 4h, 12h, 1d
	Sum       int    `json:"sum"`
	Count     int    `json:"count"`
	Max       int    `json:"max"`
	Min       int    `json:"min"`
}

// InitLedger creates the initial set of assets in the ledger.
func (t *SmartContract) InitLedger(ctx contractapi.TransactionContextInterface) error {
	logs := []Log{
		{SensorID: "asdf", Value: 600, MeasurementUnit: "HUMD", CreationTime: time.Now().Format(time.RFC3339)},
		{SensorID: "asdf", Value: 610, MeasurementUnit: "HUMD", CreationTime: time.Now().Format(time.RFC3339)},
		{SensorID: "asdf", Value: 620, MeasurementUnit: "HUMD", CreationTime: time.Now().Format(time.RFC3339)},
		{SensorID: "asdf", Value: 630, MeasurementUnit: "HUMD", CreationTime: time.Now().Format(time.RFC3339)},

		{SensorID: "fdsa", Value: 170, MeasurementUnit: "TEMP", CreationTime: time.Now().Format(time.RFC3339)},
		{SensorID: "fdsa", Value: 180, MeasurementUnit: "TEMP", CreationTime: time.Now().Format(time.RFC3339)},
		{SensorID: "fdsa", Value: 190, MeasurementUnit: "TEMP", CreationTime: time.Now().Format(time.RFC3339)},
	}

	for _, log := range logs {
		t.SetSensorState(ctx, log.SensorID, log.Value, log.MeasurementUnit, log.CreationTime)
	}

	return nil
}

func (t *SmartContract) SetSensorState(ctx contractapi.TransactionContextInterface, deviceId string, value int, measurementUnit string, creationTimeRFC3339 string) error {
	creationTime, err := time.Parse(time.RFC3339, creationTimeRFC3339)
	if err != nil {
		return err
	}

	timeframe := creationTime.Format("2006-01-02T15:04")
	updateAggregation(ctx, deviceId, timeframe, value)
	timeframe = creationTime.Format("2006-01-02T15")
	updateAggregation(ctx, deviceId, timeframe, value)
	timeframe = creationTime.Format("2006-01-02")
	updateAggregation(ctx, deviceId, timeframe, value)
	timeframe = creationTime.Format("2006-01")
	updateAggregation(ctx, deviceId, timeframe, value)

	log := &Log{
		SensorID:        deviceId,
		CreationTime:    creationTime.Format(time.RFC3339),
		Value:           value,
		MeasurementUnit: measurementUnit,
	}
	assetBytes, err := json.Marshal(log)
	if err != nil {
		return err
	}
	err = ctx.GetStub().PutState(deviceId, assetBytes)
	if err != nil {
		return err
	}
	return nil
}

func updateAggregation(ctx contractapi.TransactionContextInterface, sensorId string, timeframe string, value int) error {
	objectKey := "sensor~timeframe"
	iterator, err := ctx.GetStub().GetStateByPartialCompositeKey(objectKey, []string{sensorId, timeframe})
	if err != nil {
		return err
	}
	defer iterator.Close()
	var aggregationJSON Aggregation
	if iterator.HasNext() {
		res, err := iterator.Next()
		if err != nil {
			return err
		}
		err = json.Unmarshal(res.Value, &aggregationJSON)
		if err != nil {
			return err
		}
		aggregationJSON.Sum += value
		aggregationJSON.Count += 1
		if aggregationJSON.Min > value {
			aggregationJSON.Min = value
		}
		if aggregationJSON.Max < value {
			aggregationJSON.Max = value
		}
	} else {
		aggregationJSON = Aggregation{
			SensorID:  sensorId,
			TimeFrame: timeframe,
			Count:     1,
			Sum:       value,
			Min:       value,
			Max:       value,
		}
	}
	key, err := ctx.GetStub().CreateCompositeKey(objectKey, []string{sensorId, timeframe})
	if err != nil {
		return err
	}
	bytes, err := json.Marshal(aggregationJSON)
	if err != nil {
		return err
	}
	return ctx.GetStub().PutState(key, bytes)
}

func (t *SmartContract) GetAggregation(ctx contractapi.TransactionContextInterface, sensorId string, timeframe string) ([]*Aggregation, error) {
	objectKey := "sensor~timeframe"
	iterator, err := ctx.GetStub().GetStateByPartialCompositeKey(objectKey, []string{sensorId, timeframe})
	if err != nil {
		return nil, err
	}
	return constructQueryResponseFromIteratorAggregator(iterator)
}

func (t *SmartContract) GetHistoryForKeyCount(ctx contractapi.TransactionContextInterface, id string) (string, error) {
	resultsIterator, err := ctx.GetStub().GetHistoryForKey(id)
	if err != nil {
		return err.Error(), err
	}
	defer resultsIterator.Close()

	// buffer is a JSON array containing historic values for the marble
	size := 0
	for resultsIterator.HasNext() {
		_, err := resultsIterator.Next()
		if err != nil {
			return err.Error(), err
		}
		size = size + 1
	}

	return strconv.Itoa(size), nil
}

func (t *SmartContract) GetHistoryForKey(ctx contractapi.TransactionContextInterface, id string) (string, error) {
	resultsIterator, err := ctx.GetStub().GetHistoryForKey(id)
	if err != nil {
		return err.Error(), err
	}
	defer resultsIterator.Close()

	// buffer is a JSON array containing historic values for the marble
	var buffer bytes.Buffer
	buffer.WriteString("[")

	bArrayMemberAlreadyWritten := false
	for resultsIterator.HasNext() {
		response, err := resultsIterator.Next()
		if err != nil {
			return err.Error(), err
		}
		// Add a comma before array members, suppress it for the first array member
		if bArrayMemberAlreadyWritten == true {
			buffer.WriteString(",")
		}
		writeToBuffer(&buffer, response)
		bArrayMemberAlreadyWritten = true
	}
	buffer.WriteString("]")

	fmt.Printf("- getHistoryForMarble returning:\n%s\n", buffer.String())

	return string(buffer.Bytes()), nil
}

func (t *SmartContract) Get1(ctx contractapi.TransactionContextInterface, id string, timeframe1 string, timeframe2 string, timeframe3 string) (string, error) {
	resultsIterator, err := ctx.GetStub().GetHistoryForKey(id)
	if err != nil {
		return err.Error(), err
	}
	defer resultsIterator.Close()

	// buffer is a JSON array containing historic values for the marble
	var buffer bytes.Buffer
	buffer.WriteString("[")

	bArrayMemberAlreadyWritten := false
	for resultsIterator.HasNext() {
		response, err := resultsIterator.Next()
		if err != nil {
			return err.Error(), err
		}
		if strings.HasPrefix(time.Unix(response.Timestamp.Seconds, 0).Format(time.RFC3339), timeframe1) ||
			strings.HasPrefix(time.Unix(response.Timestamp.Seconds, 0).Format(time.RFC3339), timeframe2) ||
			strings.HasPrefix(time.Unix(response.Timestamp.Seconds, 0).Format(time.RFC3339), timeframe3) {
			// Add a comma before array members, suppress it for the first array member
			if bArrayMemberAlreadyWritten == true {
				buffer.WriteString(",")
			}
			writeToBuffer(&buffer, response)
			bArrayMemberAlreadyWritten = true
		}
	}
	buffer.WriteString("]")

	fmt.Printf("- get1 returning:\n%s\n", buffer.String())

	return string(buffer.Bytes()), nil
}

func (t *SmartContract) Get2(ctx contractapi.TransactionContextInterface, id string, valueMin string, valueMax string) (string, error) {

	min, err := strconv.Atoi(valueMin)
	if err != nil {
		return err.Error(), err
	}
	max, err := strconv.Atoi(valueMax)
	if err != nil {
		return err.Error(), err
	}

	resultsIterator, err := ctx.GetStub().GetHistoryForKey(id)
	if err != nil {
		return err.Error(), err
	}
	defer resultsIterator.Close()

	// buffer is a JSON array containing historic values for the marble
	var buffer bytes.Buffer
	buffer.WriteString("[")

	bArrayMemberAlreadyWritten := false
	for resultsIterator.HasNext() {
		response, err := resultsIterator.Next()
		if err != nil {
			return err.Error(), err
		}

		var log Log
		err = json.Unmarshal(response.Value, &log)

		if log.Value > min && log.Value < max {
			// Add a comma before array members, suppress it for the first array member
			if bArrayMemberAlreadyWritten == true {
				buffer.WriteString(",")
			}
			writeToBuffer(&buffer, response)
			bArrayMemberAlreadyWritten = true
		}
	}
	buffer.WriteString("]")

	fmt.Printf("- get1 returning:\n%s\n", buffer.String())

	return string(buffer.Bytes()), nil
}

func (t *SmartContract) Get3(ctx contractapi.TransactionContextInterface, id string, createdFrom string, createdTo string) (string, error) {

	from, err := parseTime(createdFrom)
	if err != nil {
		return err.Error(), err
	}

	to, err := parseTime(createdTo)
	if err != nil {
		return err.Error(), err
	}

	resultsIterator, err := ctx.GetStub().GetHistoryForKey(id)
	if err != nil {
		return err.Error(), err
	}
	defer resultsIterator.Close()

	// buffer is a JSON array containing historic values for the marble
	var buffer bytes.Buffer
	buffer.WriteString("[")

	bArrayMemberAlreadyWritten := false
	for resultsIterator.HasNext() {
		response, err := resultsIterator.Next()
		if err != nil {
			return err.Error(), err
		}

		var log Log
		err = json.Unmarshal(response.Value, &log)
		createdAt, err := time.Parse(time.RFC3339, log.CreationTime)

		if createdAt.After(from) && createdAt.Before(to) {
			// Add a comma before array members, suppress it for the first array member
			if bArrayMemberAlreadyWritten == true {
				buffer.WriteString(",")
			}
			writeToBuffer(&buffer, response)
			bArrayMemberAlreadyWritten = true
		}
	}
	buffer.WriteString("]")

	fmt.Printf("- get1 returning:\n%s\n", buffer.String())

	return string(buffer.Bytes()), nil
}

func parseTime(input string) (time.Time, error) {
	res, err := time.Parse("2006", input)
	if err == nil {
		return res, nil
	}
	res, err = time.Parse("2006-01", input)
	if err == nil {
		return res, nil
	}
	res, err = time.Parse("2006-01-02", input)
	if err == nil {
		return res, nil
	}
	res, err = time.Parse("2006-01-02T15", input)
	if err == nil {
		return res, nil
	}
	res, err = time.Parse("2006-01-02T15:04", input)
	if err == nil {
		return res, nil
	}
	res, err = time.Parse("2006-01-02T15:04:05", input)
	if err == nil {
		return res, nil
	}
	res, err = time.Parse(time.RFC3339, input)
	if err == nil {
		return res, nil
	}
	return res, err
}

func writeToBuffer(buffer *bytes.Buffer, response *queryresult.KeyModification) {
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

func constructQueryResponseFromIteratorAggregator(resultsIterator shim.StateQueryIteratorInterface) ([]*Aggregation, error) {
	var aggs []*Aggregation
	for resultsIterator.HasNext() {

		queryResult, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}
		var agg Aggregation
		err = json.Unmarshal(queryResult.Value, &agg)

		if err != nil {
			return nil, err
		}

		aggs = append(aggs, &agg)

	}
	return aggs, nil
}
