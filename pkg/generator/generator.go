package generator

import (
	"errors"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/txnbuild"
	"github.com/stellot/stellot-iot/pkg/crypto"
	"github.com/stellot/stellot-iot/pkg/helpers"
	"github.com/stellot/stellot-iot/pkg/usecases"
	"golang.org/x/time/rate"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type SensorDevice struct {
	DeviceId      int
	LogValue      [32]byte
	PhysicsType   usecases.PhysicsType
	Server        string
	Horizon       *horizonclient.Client
	DeviceKeypair *keypair.Full
	Account       *horizon.Account
	RateLimiter   *rate.Limiter
}
type SendLogResult struct {
	HTTPResponseBody string
	HorizonResponse  *horizon.Transaction
	Error            error
}

func SendLogTx(params SensorDevice, eventIndex int) SendLogResult {
	seqNum, err := strconv.ParseInt(params.Account.Sequence, 10, 64)
	if err != nil {
		return SendLogResult{Error: err}
	}

	logValue := params.PhysicsType.RandomValue(eventIndex + params.DeviceId)
	payload, err := crypto.EncryptToMemo(seqNum+1, params.DeviceKeypair, helpers.BatchKeypair.Address(), logValue)
	memo := txnbuild.MemoHash(*payload)

	txParams := txnbuild.TransactionParams{
		SourceAccount:        params.Account,
		IncrementSequenceNum: true,
		Operations: []txnbuild.Operation{&txnbuild.Payment{
			Destination: helpers.BatchKeypair.Address(),
			Asset:       params.PhysicsType.Asset(),
			Amount:      "0.0000001",
		}},
		Memo:       memo,
		Timebounds: txnbuild.NewTimebounds(time.Now().UTC().Unix(), txnbuild.TimeoutInfinite),
		BaseFee:    100,
	}

	tx, err := txnbuild.NewTransaction(txParams)
	if err != nil {
		log.Println("Error creating new transaction", err)
		return SendLogResult{Error: err}
	}
	signedTx, err := tx.Sign(helpers.NetworkPassphrase, params.DeviceKeypair)
	if err != nil {
		log.Println("Error signing transaction", err)
		return SendLogResult{Error: err}
	}
	xdr, err := signedTx.Base64()
	if err != nil {
		log.Println("Error converting to base64", err)
		return SendLogResult{Error: err}
	}

	if helpers.SendTxTo == "horizon" {
		resp, err := sendTxToHorizon(params.Horizon, signedTx)
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
		log.Printf("Success sending log deviceId %d log no. %d %s", params.DeviceId, eventIndex, string(resp.ResultXdr))
		return SendLogResult{HorizonResponse: &resp, Error: err}
	} else if helpers.SendTxTo == "stellar-core" {
		body, err := bruteForceTransaction(params, xdr, eventIndex)
		return SendLogResult{HTTPResponseBody: string(body), Error: err}
	} else {
		return SendLogResult{Error: errors.New("Unsupported sendTxTo")}
	}
}

func bruteForceTransaction(params SensorDevice, xdr string, eventIndex int) (string, error) {
	for {
		response, err := sendTxToStellarCore(params.Server, xdr)
		if err != nil {
			uError := err.(*url.Error)
			log.Printf("Error sending get request to stellar core %+v\n", uError)
		}
		defer response.Body.Close()
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Printf("Error reading body of log device: %d log no. %d %v", params.DeviceId, eventIndex, err)
		} else {
			if !strings.Contains(string(body), "ERROR") {
				log.Printf("Success sending log deviceId %d log no. %d %s", params.DeviceId, eventIndex, string(body))
				return string(body), nil
			} else {
				if strings.Contains(string(body), "AAAAAAAAAAH////7AAAAAA==") {
					log.Println("Received bad seq error, Retrying in 1sec")
					time.Sleep(1 * time.Second)
				} else {
					log.Fatalf("Received ERROR transactioin in deviceId %d log no. %d", params.DeviceId, eventIndex)
				}
			}
		}
	}
}

func sendTxToHorizon(horizon *horizonclient.Client, transaction *txnbuild.Transaction) (horizon.Transaction, error) {
	return horizon.SubmitTransactionWithOptions(transaction, horizonclient.SubmitTxOpts{SkipMemoRequiredCheck: true})
}

func sendTxToStellarCore(server string, xdr string) (resp *http.Response, err error) {
	req, err := http.NewRequest("GET", server+"/tx", nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Add("blob", xdr)
	req.URL.RawQuery = q.Encode()
	return http.Get(req.URL.String())
}

type LoadAccountResult struct {
	Account *horizon.Account
	Error   error
}

func CreateSensorDevices(keypairs []*keypair.Full) []SensorDevice {
	channels := make([]chan LoadAccountResult, len(keypairs))
	for i := 0; i < len(channels); i++ {
		channels[i] = loadAccountChan(keypairs[i].Address())
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
		if rand.Intn(2) == 0 {
			physicType = usecases.HUMD
		}

		iotDevices[i] = SensorDevice{
			DeviceId:      i,
			PhysicsType:   physicType,
			Server:        helpers.RandomStellarCoreUrl(),
			Horizon:       helpers.RandomHorizon(),
			DeviceKeypair: keypairs[i],
			Account:       result.Account,
			RateLimiter:   rate.NewLimiter(rate.Every(time.Duration(1000.0/helpers.Tps)*time.Millisecond), 1),
		}
	}
	return iotDevices
}

func loadAccountChan(accountId string) chan LoadAccountResult {
	ch := make(chan LoadAccountResult)
	accReq := horizonclient.AccountRequest{AccountID: accountId}
	go func() {
		masterAccount, err := helpers.RandomHorizon().AccountDetail(accReq)
		if err != nil {
			ch <- LoadAccountResult{Account: nil, Error: err}
		} else {
			ch <- LoadAccountResult{Account: &masterAccount, Error: nil}
		}
	}()
	return ch
}
