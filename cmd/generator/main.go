package main

import (
	"context"
	"log"
	"math/rand"
	"net/http"
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
	if err != nil {
		log.Fatal(err)
	}

	res, err := createAccounts([]*keypair.Full{helpers.BatchKeypair}, helpers.MasterKp, masterAccount, helpers.RandomHorizon())
	if err != nil {
		hError, ok := err.(*horizonclient.Error)
		if ok {
			log.Println("Error submitting create batch account tx:", hError.Problem.Extras["result_codes"])
		} else {
			log.Println("Error submitting create batch account tx:", err)
		}
	}
	if res != nil {
		log.Println(res.Successful)
	}

	res, err = createAccounts([]*keypair.Full{usecases.HumdAssetKeypair, usecases.TempAssetKeypair}, helpers.MasterKp, masterAccount, helpers.RandomHorizon())
	if err != nil {
		hError, ok := err.(*horizonclient.Error)
		if ok {
			log.Println("Error submitting create auxiliary accounts tx:", hError.Problem.Extras["result_codes"])
		} else {
			log.Println("Error submitting create auxiliary accounts tx:", err)
		}
	}
	if res != nil {
		log.Println(res.Successful)
	}

	createSensorAccounts(keypairs, masterAccount)
	iotDevices := generator.CreateSensorDevices(keypairs)
	fundTokensToSensors(keypairs)

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

func fundTokensToSensors(keypairs []*keypair.Full) {
	//TODO

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
