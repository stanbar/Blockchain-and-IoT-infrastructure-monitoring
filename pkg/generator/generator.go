package generator

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/txnbuild"
	"github.com/stellot/stellot-iot/pkg/crypto"
	"github.com/stellot/stellot-iot/pkg/helpers"
	"github.com/stellot/stellot-iot/pkg/usecases"
	"golang.org/x/time/rate"
)

type SensorDevice struct {
	DeviceId    int
	LogValue    [32]byte
	PhysicsType usecases.PhysicsType
	Server      string
	Horizon     *horizonclient.Client
	keypair     *keypair.Full
	account     *horizon.Account
	RateLimiter *rate.Limiter
}

func (s SensorDevice) Keypair() *keypair.Full {
	return s.keypair
}

func (s SensorDevice) Account() *horizon.Account {
	return s.account
}

type SendLogResult struct {
	HTTPResponseBody string
	HorizonResponse  *horizon.Transaction
	Error            error
}

func SendLogTxToHorizon(params SensorDevice, eventIndex int) SendLogResult {
	seqNum, err := strconv.ParseInt(params.Account().Sequence, 10, 64)
	if err != nil {
		return SendLogResult{Error: err}
	}

	logValue := params.PhysicsType.RandomValue(eventIndex + params.DeviceId)
	payload, err := crypto.EncryptToMemo(seqNum+1, params.Keypair(), helpers.BatchKeypair.Address(), logValue)
	memo := txnbuild.MemoHash(*payload)

	if helpers.SendTxTo == "horizon" {
		txParams := txnbuild.TransactionParams{
			SourceAccount:        params.Account(),
			IncrementSequenceNum: true,
			Operations: []txnbuild.Operation{&txnbuild.Payment{
				Destination: helpers.BatchKeypair.Address(),
				Asset:       params.PhysicsType.Asset(),
				Amount:      "0.0000001",
			}},
			Memo:       memo,
			Timebounds: txnbuild.NewTimebounds(time.Now().UTC().Unix()-100000, txnbuild.TimeoutInfinite),
			BaseFee:    100,
		}

		tx, err := txnbuild.NewTransaction(txParams)
		if err != nil {
			log.Println("Error creating new transaction", err)
			return SendLogResult{Error: err}
		}
		signedTx, err := tx.Sign(helpers.NetworkPassphrase, params.Keypair())
		if err != nil {
			log.Println("Error signing transaction", err)
			return SendLogResult{Error: err}
		}
		resp, err := helpers.SendTxToHorizon(signedTx)
		if err != nil {
			hError := err.(*horizonclient.Error)
			if hError.Problem.Extras != nil {
				if hError.Problem.Extras["result_codes"] != nil {
					log.Fatalf("Error submitting sendLogTx to horizon, log device: %d log no. %d error: %v", params.DeviceId, eventIndex, hError.Problem.Extras["result_codes"])
				} else {
					log.Fatalf("Error submitting sendLogTx to horizon, log device: %d log no. %d error: %v", params.DeviceId, eventIndex, hError.Problem.Extras)
				}
			} else {
				log.Fatalf("Error submitting sendLogTx to horizon, log device: %d log no. %d error: %s", params.DeviceId, eventIndex, err)
			}
		}
		log.Printf("Success sending log deviceId %02d log no. %d %s", params.DeviceId, eventIndex, string(resp.ResultXdr))
		return SendLogResult{HorizonResponse: &resp, Error: err}
	} else if helpers.SendTxTo == "stellar-core" {
		body, err := sendLogToStellarCode(params, eventIndex, memo)
		return SendLogResult{HTTPResponseBody: string(body), Error: err}
	} else {
		return SendLogResult{Error: errors.New("Unsupported sendTxTo")}
	}
}

func sendLogToStellarCode(params SensorDevice, eventIndex int, memo txnbuild.MemoHash) (string, error) {
	txParams := txnbuild.TransactionParams{
		SourceAccount:        params.Account(),
		IncrementSequenceNum: true,
		Operations: []txnbuild.Operation{&txnbuild.Payment{
			Destination: helpers.BatchKeypair.Address(),
			Asset:       params.PhysicsType.Asset(),
			Amount:      "0.0000001",
		}},
		Memo:       memo,
		Timebounds: txnbuild.NewTimebounds(time.Now().UTC().Unix()-100000, txnbuild.TimeoutInfinite),
		BaseFee:    100,
	}
	tx, err := txnbuild.NewTransaction(txParams)
	if err != nil {
		log.Fatalln("Error creating new transaction", err)
	}
	signedTx, err := tx.Sign(helpers.NetworkPassphrase, params.Keypair())
	if err != nil {
		log.Fatalln("Error signing transaction", err)
	}
	xdr, err := signedTx.Base64()
	response, err := helpers.SendTxToStellarCore(params.Server, xdr)
	if err != nil {
		uError := err.(*url.Error)
		log.Printf("Error sending get request to stellar core %+v\n", uError)
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Printf("Error reading body of log device: %d log no. %d %v", params.DeviceId, eventIndex, err)
		return "", err
	} else {
		if !strings.Contains(string(body), "ERROR") {
			log.Printf("Success sending log deviceId %02d log no. %d %s", params.DeviceId, eventIndex, string(body))
			return string(body), nil
		} else {
			if strings.Contains(string(body), "AAAAAAAAAAH////7AAAAAA==") {
				errMessage := fmt.Sprintf("Received bad seq error, deviceId %d log no. %d Retrying in 1sec", params.DeviceId, eventIndex)
				log.Println(errMessage)
				return string(body), errors.New(errMessage)
			} else {
				log.Fatalf("Received ERROR transactioin in deviceId %d log no. %d", params.DeviceId, eventIndex)
				return string(body), errors.New(fmt.Sprintf("Received bad seq error, deviceId %d log no. %d Retrying in 1sec", params.DeviceId, eventIndex))
			}
		}
	}
}

func CreateSensorDevices(keypairs []*keypair.Full) []SensorDevice {
	channels := make([]chan helpers.LoadAccountResult, len(keypairs))
	for i := 0; i < len(channels); i++ {
		channels[i] = helpers.LoadAccountChan(keypairs[i].Address())
	}

	iotDevices := make([]SensorDevice, len(keypairs))
	for i := 0; i < len(keypairs); i++ {
		result := <-channels[i]
		if result.Error != nil {
			log.Println(result.Error)
			hError := result.Error.(*horizonclient.Error)
			log.Println("Error submitting transaction:", hError.Problem.Extras["result_codes"])
		}

		physicType := usecases.TEMP
		if i <= len(keypairs) {
			physicType = usecases.HUMD
		}

		iotDevices[i] = SensorDevice{
			DeviceId:    i,
			PhysicsType: physicType,
			Server:      helpers.RandomStellarCoreUrl(),
			Horizon:     helpers.RandomHorizon(),
			keypair:     keypairs[i],
			account:     result.Account,
			RateLimiter: rate.NewLimiter(rate.Every(time.Duration(1000.0/helpers.Tps)*time.Millisecond), 1),
		}
	}
	return iotDevices
}
