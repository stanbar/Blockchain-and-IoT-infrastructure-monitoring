package main

import (
	"context"
	"log"
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
	keypairs := helpers.DevicesKeypairs
	masterAccount, err := helpers.LoadMasterAccount()
	if err != nil {
		log.Fatal(err)
	}

	helpers.HandleGracefuly(helpers.CreateAccounts([]*keypair.Full{helpers.BatchKeypair, usecases.AssetKeypair}, helpers.RandomHorizon()))
	batchAcc, err := helpers.LoadAccount(helpers.BatchKeypair.Address())
	if err != nil {
		log.Fatal(err)
	}
	assetAccount, err := helpers.LoadAccount(usecases.AssetIssuer)
	if err != nil {
		log.Fatal(err)
	}

	createSensorAccountsSilently(keypairs, masterAccount)

	helpers.HandleGracefuly(createReceiverTrustlines(batchAcc, helpers.BatchKeypair))

	iotDevices := generator.CreateSensorDevices(keypairs)
	devices := make([]helpers.CreateTrustline, len(iotDevices))
	for i, v := range iotDevices {
		devices[i] = v
	}
	helpers.HandleGracefuly(helpers.CreateTrustlines(devices, []txnbuild.Asset{usecases.HUMD.Asset()}))

	helpers.HandleGracefuly(fundTokensToSensors(iotDevices, assetAccount, usecases.AssetKeypair))

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

func createSensorAccountsSilently(keypairs []*keypair.Full, masterAccount *horizon.Account) {
	chunks := utils.ChunkKeypairs(keypairs, 100)
	for _, chunk := range chunks {
		helpers.HandleGracefuly(helpers.CreateAccounts(chunk, helpers.RandomHorizon()))
	}
}

func createReceiverTrustlines(receiverAcc *horizon.Account, receiverKeypair *keypair.Full) error {
	ops := []txnbuild.Operation{&txnbuild.ChangeTrust{
		Line:          usecases.TEMP.Asset(),
		SourceAccount: receiverAcc,
	}, &txnbuild.ChangeTrust{
		Line:          usecases.HUMD.Asset(),
		SourceAccount: receiverAcc,
	}}

	txParams := txnbuild.TransactionParams{
		SourceAccount:        receiverAcc,
		IncrementSequenceNum: true,
		Operations:           ops,
		Timebounds:           txnbuild.NewTimeout(120),
		BaseFee:              100,
	}

	tx, err := txnbuild.NewTransaction(txParams)
	if err != nil {
		return err
	}

	signedTx, err := tx.Sign(helpers.NetworkPassphrase, helpers.MasterKp)
	if err != nil {
		return err
	}

	signedTx, err = tx.Sign(helpers.NetworkPassphrase, receiverKeypair)
	if err != nil {
		return err
	}

	log.Println("Submitting createReceiverTrustlines transaction")
	_, err = helpers.RandomHorizon().SubmitTransactionWithOptions(signedTx, horizonclient.SubmitTxOpts{SkipMemoRequiredCheck: true})
	return err
}

func ChunkDevices(slice []generator.SensorDevice, chunkSize int) [][]generator.SensorDevice {
	var chunks [][]generator.SensorDevice
	for {
		if len(slice) == 0 {
			break
		}
		// necessary check to avoid slicing beyond
		// slice capacity
		if len(slice) < chunkSize {
			chunkSize = len(slice)
		}
		chunks = append(chunks, slice[0:chunkSize])
		slice = slice[chunkSize:]
	}
	return chunks
}

func fundTokensToSensors(devices []generator.SensorDevice, assetAccount *horizon.Account, assetKeypair *keypair.Full) error {
	ops := make([]txnbuild.Operation, len(devices))

	// https://developers.stellar.org/docs/issuing-assets/anatomy-of-an-asset/#amount-precision
	// ((2^63)-1)/(10^7) = 922,337,203,685.4775807
	maxValue, err := strconv.ParseInt("9223372036854775807", 10, 64)
	if err != nil {
		log.Fatal("Can not parse max asset value")
	}
	amount := strconv.FormatInt(maxValue/int64(len(devices)), 10)
	separatorIndex := len(amount) - 7

	for i, v := range devices {
		ops[i] = &txnbuild.Payment{
			Asset:       v.PhysicsType.Asset(),
			Destination: v.Account().AccountID,
			Amount:      amount[:separatorIndex] + "." + amount[separatorIndex:],
		}
	}

	txParams := txnbuild.TransactionParams{
		SourceAccount:        assetAccount,
		IncrementSequenceNum: true,
		Operations:           ops,
		Timebounds:           txnbuild.NewTimeout(120),
		BaseFee:              100,
	}

	tx, err := txnbuild.NewTransaction(txParams)
	if err != nil {
		return err
	}

	signedTx, err := tx.Sign(helpers.NetworkPassphrase, assetKeypair)
	if err != nil {
		return err
	}

	log.Println("Submitting fundTokensToSensors transaction")
	_, err = helpers.RandomHorizon().SubmitTransactionWithOptions(signedTx, horizonclient.SubmitTxOpts{SkipMemoRequiredCheck: true})
	return err
}
