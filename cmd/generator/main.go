package main

import (
	"context"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

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
	masterAcc := helpers.MustLoadMasterAccount()
	keypairs := helpers.DevicesKeypairs

	helpers.MustCreateAccounts(masterAcc, []*keypair.Full{helpers.BatchKeypair, usecases.AssetKeypair})

	time.Sleep(5000) // close the ledger

	batchAcc := helpers.MustLoadAccount(helpers.BatchKeypair.Address())
	assetAccount := helpers.MustLoadAccount(usecases.AssetIssuer)
	createSensorAccountsSilently(masterAcc, keypairs)

	tryCreateReceiverTrustlines(masterAcc, batchAcc, helpers.BatchKeypair)

	iotDevices := generator.CreateSensorDevices(keypairs)
	devices := make([]helpers.CreateTrustline, len(iotDevices))
	for i, v := range iotDevices {
		devices[i] = v
	}
	helpers.TryCreateTrustlines(masterAcc, devices, []txnbuild.Asset{usecases.HUMD.Asset(), usecases.TEMP.Asset()})

	tryFundTokensToSensors(masterAcc, iotDevices, assetAccount, usecases.AssetKeypair)

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

func createSensorAccountsSilently(masterAcc *horizon.Account, keypairs []*keypair.Full) {
	chunks := utils.ChunkKeypairs(keypairs, 100)
	for _, chunk := range chunks {
		helpers.MustCreateAccounts(masterAcc, chunk)
	}
}

func tryCreateReceiverTrustlines(masterAcc *horizon.Account, receiverAcc *horizon.Account, receiverKeypair *keypair.Full) {
	ops := []txnbuild.Operation{&txnbuild.ChangeTrust{
		Line:          usecases.TEMP.Asset(),
		SourceAccount: receiverAcc,
	}, &txnbuild.ChangeTrust{
		Line:          usecases.HUMD.Asset(),
		SourceAccount: receiverAcc,
	}}
	helpers.MustSendTransactionFromMasterKey(masterAcc, ops, receiverKeypair)
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

func tryFundTokensToSensors(masterAcc *horizon.Account, devices []generator.SensorDevice, assetAccount *horizon.Account, assetKeypair *keypair.Full) {

	// https://developers.stellar.org/docs/issuing-assets/anatomy-of-an-asset/#amount-precision
	// ((2^63)-1)/(10^7) = 922,337,203,685.4775807
	maxValue, err := strconv.ParseInt("9223372036854775807", 10, 64)
	if err != nil {
		log.Fatal("Can not parse max asset value")
	}
	amount := strconv.FormatInt(maxValue/int64(len(devices)), 10)
	separatorIndex := len(amount) - 7

	ops := make([]txnbuild.Operation, len(devices))
	for i, v := range devices {
		ops[i] = &txnbuild.Payment{
			Asset:         v.PhysicsType.Asset(),
			Destination:   v.Account().AccountID,
			Amount:        amount[:separatorIndex] + "." + amount[separatorIndex:],
			SourceAccount: assetAccount,
		}
	}
	helpers.MustSendTransactionFromMasterKey(masterAcc, ops, assetKeypair)
}
