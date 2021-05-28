package chaincode_test

import (
	"fmt"
	"testing"

	"github.com/hyperledger/fabric-chaincode-go/shim"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
	"github.com/stellot/stellot-iot/fabric-smart-contract/chaincode"
	"github.com/stellot/stellot-iot/fabric-smart-contract/chaincode/mocks"
	"github.com/stretchr/testify/require"
)

type transactionContext interface {
	contractapi.TransactionContextInterface
}

type chaincodeStub interface {
	shim.ChaincodeStubInterface
}

type stateQueryIterator interface {
	shim.StateQueryIteratorInterface
}

func TestInitLedger(t *testing.T) {
	chaincodeStub := &mocks.ChaincodeStub{}
	transactionContext := &mocks.TransactionContext{}
	transactionContext.GetStubReturns(chaincodeStub)

	assetTransfer := chaincode.SmartContract{}
	err := assetTransfer.InitLedger(transactionContext)
	require.NoError(t, err)

	chaincodeStub.PutStateReturns(fmt.Errorf("failed inserting key"))
	err = assetTransfer.InitLedger(transactionContext)
	require.EqualError(t, err, "failed to put to world state. failed inserting key")

}
