package main

import (
	"context"
	"log"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/txnbuild"
	"github.com/stellot/stellot-iot/pkg/generator"
	"github.com/stellot/stellot-iot/pkg/helpers"
	"github.com/stellot/stellot-iot/pkg/usecases"
	"github.com/stellot/stellot-iot/pkg/utils"
)

func main() {
	http.DefaultClient.Timeout = time.Second * time.Duration(helpers.TimeOut)
	rand.Seed(time.Now().UnixNano())
	keypairs := helpers.DevicesKeypairs()
	masterAccount, err := helpers.LoadMasterAccount()
	batchAcc, err := helpers.LoadAccount(helpers.BatchKeypair.Address())
	if err != nil {
		log.Fatal(err)
	}

	createAccountSilently("batch", []*keypair.Full{helpers.BatchKeypair}, helpers.MasterKp, masterAccount, helpers.RandomHorizon())
	createAccountSilently("asset", []*keypair.Full{usecases.AssetKeypair}, helpers.MasterKp, masterAccount, helpers.RandomHorizon())

	createSensorAccounts(keypairs, masterAccount)

	iotDevices := generator.CreateSensorDevices(keypairs)
	createReceiverTrustlines(batchAcc, helpers.BatchKeypair, masterAccount)
	createAssetTrustlines(iotDevices, masterAccount)
	fundTokensToSensors(iotDevices, masterAccount)

	var wg sync.WaitGroup
	for _, iotDevice := range iotDevices {
		wg.Add(1)
		go func(params generator.SensorDevice, wg *sync.WaitGroup) {
			defer wg.Done()
			time.Sleep(time.Duration(1000.0*params.DeviceId/len(iotDevices)) * time.Millisecond)
			for i := 0; i < helpers.LogsNumber; i++ {
				ctx := context.Background()
				err := params.RateLimiter.Wait(ctx)
				if err != nil {
					log.Println("Error returned by limiter", err)
					return
				}
				generator.SendLogTx(params, i)
			}
		}(iotDevice, &wg)
	}

	log.Println("Main: Waiting for workers to finish")
	wg.Wait()
	log.Println("Main: Completed")
}

func createSensorAccounts(keypairs []*keypair.Full, masterAccount *horizon.Account) {
	chunks := utils.ChunkKeypairs(keypairs, 100)
	for _, chunk := range chunks {
		res, err := createAccounts(chunk, helpers.MasterKp, masterAccount, helpers.RandomHorizon())
		if err != nil {
			hError, ok := err.(*horizonclient.Error)
			if ok {
				log.Println("Error submitting transaction:", hError.Problem.Extras)
			}
		}
		if res != nil {
			log.Println(res.Successful)
		}
	}
}

func createReceiverTrustlines(receiverAcc *horizon.Account, receiverKeypair *keypair.Full, sourceAcc *horizon.Account) (*horizon.Transaction, error) {
	ops := []txnbuild.Operation{&txnbuild.ChangeTrust{
		Line:          usecases.TEMP.Asset(),
		SourceAccount: receiverAcc,
		Limit:         strconv.Itoa(math.MaxInt64),
	}, &txnbuild.ChangeTrust{
		Line:          usecases.HUMD.Asset(),
		SourceAccount: receiverAcc,
		Limit:         strconv.Itoa(math.MaxInt64),
	}}

	txParams := txnbuild.TransactionParams{
		SourceAccount:        sourceAcc,
		IncrementSequenceNum: true,
		Operations:           ops,
		Timebounds:           txnbuild.NewTimeout(120),
		BaseFee:              100,
	}

	tx, err := txnbuild.NewTransaction(txParams)
	if err != nil {
		return nil, err
	}

	signedTx, err := tx.Sign(helpers.NetworkPassphrase, helpers.MasterKp)
	if err != nil {
		return nil, err
	}

	signedTx, err = tx.Sign(helpers.NetworkPassphrase, receiverKeypair)
	if err != nil {
		return nil, err
	}

	log.Println("Submitting fundTokensToSensors transaction")
	response, err := helpers.RandomHorizon().SubmitTransactionWithOptions(signedTx, horizonclient.SubmitTxOpts{SkipMemoRequiredCheck: true})
	return &response, err

}

func createAssetTrustlines(devices []generator.SensorDevice, sourceAcc *horizon.Account) (*horizon.Transaction, error) {
	fundAccountsOps := make([]txnbuild.Operation, len(devices))
	for i, v := range devices {
		fundAccountsOps[i] = &txnbuild.ChangeTrust{
			Line:          v.PhysicsType.Asset(),
			SourceAccount: v.Account,
			Limit:         strconv.Itoa(math.MaxInt64),
		}
	}
	txParams := txnbuild.TransactionParams{
		SourceAccount:        sourceAcc,
		IncrementSequenceNum: true,
		Operations:           fundAccountsOps,
		Timebounds:           txnbuild.NewTimeout(120),
		BaseFee:              100,
	}

	tx, err := txnbuild.NewTransaction(txParams)
	if err != nil {
		return nil, err
	}

	signedTx, err := tx.Sign(helpers.NetworkPassphrase, helpers.MasterKp)
	if err != nil {
		return nil, err
	}
	for _, v := range devices {
		signedTx, err = tx.Sign(helpers.NetworkPassphrase, v.DeviceKeypair)
		if err != nil {
			return nil, err
		}
	}

	log.Println("Submitting fundTokensToSensors transaction")
	response, err := helpers.RandomHorizon().SubmitTransactionWithOptions(signedTx, horizonclient.SubmitTxOpts{SkipMemoRequiredCheck: true})
	return &response, err
}

func fundTokensToSensors(devices []generator.SensorDevice, sourceAcc *horizon.Account) (*horizon.Transaction, error) {
	fundAccountsOps := make([]txnbuild.Operation, len(devices))

	for i, v := range devices {
		fundAccountsOps[i] = &txnbuild.Payment{
			Asset:         v.PhysicsType.Asset(),
			Destination:   v.Account.AccountID,
			SourceAccount: v.Account,
			Amount:        strconv.Itoa(math.MaxInt64 / len(devices)),
		}
	}

	txParams := txnbuild.TransactionParams{
		SourceAccount:        sourceAcc,
		IncrementSequenceNum: true,
		Operations:           fundAccountsOps,
		Timebounds:           txnbuild.NewTimeout(120),
		BaseFee:              100,
	}

	tx, err := txnbuild.NewTransaction(txParams)
	if err != nil {
		return nil, err
	}

	signedTx, err := tx.Sign(helpers.NetworkPassphrase, usecases.AssetKeypair)
	if err != nil {
		return nil, err
	}

	log.Println("Submitting fundTokensToSensors transaction")
	response, err := helpers.RandomHorizon().SubmitTransactionWithOptions(signedTx, horizonclient.SubmitTxOpts{SkipMemoRequiredCheck: true})
	return &response, err
}

func createAccountSilently(what string, kp []*keypair.Full, signer *keypair.Full, sourceAcc *horizon.Account, client *horizonclient.Client) {
	_, err := createAccounts(kp, signer, sourceAcc, client)
	if err != nil {
		hError, ok := err.(*horizonclient.Error)
		if ok {
			log.Printf("Error submitting create %s account tx: %s\n", what, hError.Problem.Extras["result_codes"])
		} else {
			log.Printf("Error submitting create %s account tx: %s\n", what, err)
		}
	} else {
		log.Printf("Successfully created %s", what)
	}
}

func createAccounts(kp []*keypair.Full, signer *keypair.Full, sourceAcc *horizon.Account, client *horizonclient.Client) (*horizon.Transaction, error) {
	createAccountOps := make([]txnbuild.Operation, len(kp))

	for i, v := range kp {
		createAccountOps[i] = &txnbuild.CreateAccount{
			Destination: v.Address(),
			Amount:      "10",
		}
	}
	txParams := txnbuild.TransactionParams{
		SourceAccount:        sourceAcc,
		IncrementSequenceNum: true,
		Operations:           createAccountOps,
		Timebounds:           txnbuild.NewTimeout(120),
		BaseFee:              100,
	}

	tx, err := txnbuild.NewTransaction(txParams)
	if err != nil {
		return nil, err
	}
	signedTx, err := tx.Sign(helpers.NetworkPassphrase, signer)
	if err != nil {
		return nil, err
	}
	log.Println("Submitting createAccount transaction")
	response, err := client.SubmitTransactionWithOptions(signedTx, horizonclient.SubmitTxOpts{SkipMemoRequiredCheck: true})
	return &response, err
}
