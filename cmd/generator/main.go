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
	helpers.BlockUntilHorizonIsReady()
	masterAcc := helpers.MustLoadMasterAccount()
	keypairs := helpers.DevicesKeypairs

	helpers.MustCreateAccounts(masterAcc, []*keypair.Full{helpers.BatchKeypair, usecases.AssetKeypair}, "horizon")

	batchAcc := helpers.MustLoadAccount(helpers.BatchKeypair.Address())
	assetAccount := helpers.MustLoadAccount(usecases.AssetIssuer)
	tryCreateSensorAccounts(masterAcc, keypairs)

	tryCreateReceiverTrustlines(masterAcc, batchAcc, helpers.BatchKeypair, "horizon")

	iotDevices := generator.CreateSensorDevices(keypairs)
	devices := make([]helpers.CreateTrustline, len(iotDevices))
	for i, v := range iotDevices {
		devices[i] = v
	}
	helpers.MustCreateTrustlines(masterAcc, devices, []txnbuild.Asset{usecases.HUMD.Asset(), usecases.TEMP.Asset()}, "core")

	tryFundTokensToSensors(masterAcc, iotDevices, assetAccount, usecases.AssetKeypair, "horizon")

	for {
		err := startGenerator(iotDevices)
		if err == nil {
			log.Println("Successfully send all transactions")
			return
		} else {
			log.Println("Failed to send transactions try again after 1 sec")
			time.Sleep(1 * time.Second)
			log.Println("Loading sensor devices")
			iotDevices = generator.CreateSensorDevices(keypairs)
			log.Println("Loaded sensor devices")
		}
	}
}

func startGenerator(iotDevices []generator.SensorDevice) error {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, iotDevice := range iotDevices {
		wg.Add(1)
		go func(params generator.SensorDevice, wg *sync.WaitGroup) {
			defer wg.Done()
			time.Sleep(time.Duration(1000.0*params.DeviceId/len(iotDevices)) * time.Millisecond)
			for i := 0; i < helpers.LogsNumber; i++ {
				select {
				case <-ctx.Done():
					log.Printf("Sensor %d received cancelation signal, exiting \n", params.DeviceId)
					return
				default: // Default is must to avoid blocking
				}
				err := params.RateLimiter.Wait(ctx)
				if err != nil {
					cancel()
					log.Println("Error returned by limiter", err)
					return
				}
				res := generator.SendLogTxToHorizon(params, i)
				if res.Error != nil {
					cancel()
					return
				}
			}
		}(iotDevice, &wg)
	}
	log.Println("Waiting for workers to finish")
	wg.Wait()
	log.Println("Completed")
	return ctx.Err()
}

func tryCreateSensorAccounts(masterAcc *horizon.Account, keypairs []*keypair.Full) {
	chunks := utils.ChunkKeypairs(keypairs, 100)
	for _, chunk := range chunks {
		helpers.MustCreateAccounts(masterAcc, chunk, "horizon")
	}
}

func tryCreateReceiverTrustlines(masterAcc *horizon.Account, receiverAcc *horizon.Account, receiverKeypair *keypair.Full, where string) {
	ops := []txnbuild.Operation{&txnbuild.ChangeTrust{
		Line:          usecases.TEMP.Asset(),
		SourceAccount: receiverAcc,
	}, &txnbuild.ChangeTrust{
		Line:          usecases.HUMD.Asset(),
		SourceAccount: receiverAcc,
	}}
	tx := helpers.MustCreateTxFromMasterAcc(masterAcc, ops, receiverKeypair)
	if where == "core" {
		helpers.MustSendTransactionToStellarCore(tx)
	} else {
		helpers.TrySendTxToHorizon(tx)
	}
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

func tryFundTokensToSensors(masterAcc *horizon.Account, devices []generator.SensorDevice, assetAccount *horizon.Account, assetKeypair *keypair.Full, where string) {

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
	txn := helpers.MustCreateTxFromMasterAcc(masterAcc, ops, assetKeypair)
	if where == "core" {
		helpers.MustSendTransactionToStellarCore(txn)
	} else {
		helpers.TrySendTxToHorizon(txn)
	}
}
