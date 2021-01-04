package main

import (
	"errors"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/txnbuild"
	"github.com/stellot/stellot-iot/generator-go/utils"
)

var stellarCoreUrls = utils.MustGetenv("STELLAR_CORE_URLS")
var stellarCoreUrlsSlice = strings.Split(stellarCoreUrls, " ")
var networkPassphrase = utils.MustGetenv("NETWORK_PASSPHRASE")
var horizonServerUrls = utils.MustGetenv("HORIZON_SERVER_URLS")
var horizonServerUrlsSlice = strings.Split(horizonServerUrls, " ")
var logsNumber, _ = strconv.Atoi(utils.MustGetenv("LOGS_NUMBER"))
var noDevices, _ = strconv.Atoi(utils.MustGetenv("NO_DEVICES"))
var peroid, _ = strconv.Atoi(utils.MustGetenv("PEROID"))
var sendTxTo = utils.MustGetenv("SEND_TX_TO")
var tps, _ = strconv.Atoi(utils.MustGetenv("TPS"))
var masterKp, _ = keypair.FromRawSeed(network.ID(networkPassphrase))
var batchKeypair = keypair.MustParseFull(utils.MustGetenv("BATCH_SECRET_KEY"))
var horizonServers = createHorizonServers()

func createHorizonServers() []*horizonclient.Client {
	horizons := make([]*horizonclient.Client, len(horizonServerUrlsSlice))
	for i, v := range horizonServerUrlsSlice {
		horizons[i] = &horizonclient.Client{
			HorizonURL: v,
		}
	}
	return horizons
}

func randomHorizon() *horizonclient.Client {
	return horizonServers[rand.Intn(len(horizonServers))]
}

func randomStellarCoreUrl() string {
	return stellarCoreUrlsSlice[rand.Intn(len(stellarCoreUrlsSlice))]
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
	signedTx, err := tx.Sign(networkPassphrase, signer)
	if err != nil {
		return nil, err
	}
	log.Println("Submitting createAccount transaction")
	response, err := client.SubmitTransactionWithOptions(signedTx, horizonclient.SubmitTxOpts{SkipMemoRequiredCheck: true})
	return &response, err
}

type SendLogResult struct {
	HTTPResponse    *http.Response
	HorizonResponse *horizon.Transaction
	Error           error
}

func sendLogTx(
	deviceId int,
	index int,
	server string,
	batchAddress *keypair.Full,
	iotDeviceKeypair *keypair.Full,
	account *horizon.Account,
	client *horizonclient.Client,
) chan SendLogResult {
	ch := make(chan SendLogResult)
	go func() {
		txParams := txnbuild.TransactionParams{
			SourceAccount:        account,
			IncrementSequenceNum: true,
			Operations: []txnbuild.Operation{&txnbuild.Payment{
				Destination: batchKeypair.Address(),
				Asset:       txnbuild.NativeAsset{},
				Amount:      "0.0000001",
			}},
			Memo:       txnbuild.MemoText(strconv.Itoa(index) + strconv.Itoa(deviceId)),
			Timebounds: txnbuild.NewTimeout(20),
			BaseFee:    100,
		}

		tx, err := txnbuild.NewTransaction(txParams)
		if err != nil {
			ch <- SendLogResult{Error: err}
			return
		}
		signedTx, err := tx.Sign(networkPassphrase, iotDeviceKeypair)
		if err != nil {
			ch <- SendLogResult{Error: err}
			return
		}
		xdr, err := signedTx.Base64()
		log.Println("xdr", xdr)
		if err != nil {
			ch <- SendLogResult{Error: err}
			return
		}

		if sendTxTo == "horizon" {
			resp, err := client.SubmitTransactionXDR(xdr)
			if err != nil {
				hError := err.(*horizonclient.Error)
				if hError.Problem.Extras != nil {
					if hError.Problem.Extras["result_codes"] != nil {
						log.Println("Error submitting sendLogTx to horizon", hError.Problem.Extras["result_codes"])
					} else {
						log.Println("Error submitting sendLogTx to horizon", hError.Problem.Extras)
					}
				}
				ch <- SendLogResult{Error: err}
				return
			}
			ch <- SendLogResult{HorizonResponse: &resp, Error: err}
		} else if sendTxTo == "stellar-core" {
			log.Println("Submitting sendLogTx transaction")
			req, err := http.NewRequest("GET", server+"/tx", nil)
			if err != nil {
				ch <- SendLogResult{Error: err}
				return
			}
			q := req.URL.Query()
			q.Add("blob", xdr)
			req.URL.RawQuery = q.Encode()
			log.Println(req.URL.String())
			response, err := http.Get(req.URL.String())
			ch <- SendLogResult{HTTPResponse: response, Error: err}
		} else {
			ch <- SendLogResult{Error: errors.New("Unsupported sendTxTo")}
		}
	}()
	return ch
}

func loadAccount(accountId string) (*horizon.Account, error) {
	accReq := horizonclient.AccountRequest{AccountID: accountId}
	masterAccount, err := randomHorizon().AccountDetail(accReq)
	if err != nil {
		return nil, err
	}
	return &masterAccount, nil
}

type LoadAccountResult struct {
	Account *horizon.Account
	Error   error
}

func loadAccountChan(accountId string) chan LoadAccountResult {
	ch := make(chan LoadAccountResult)
	accReq := horizonclient.AccountRequest{AccountID: accountId}
	go func() {
		log.Println("Sending account details request")
		masterAccount, err := randomHorizon().AccountDetail(accReq)
		if err != nil {
			ch <- LoadAccountResult{Account: nil, Error: err}
		} else {
			ch <- LoadAccountResult{Account: &masterAccount, Error: nil}
		}
	}()
	return ch
}

func loadMasterAccount() (*horizon.Account, error) {
	return loadAccount(masterKp.Address())
}

func chunkSlice(slice []*keypair.Full, chunkSize int) [][]*keypair.Full {
	var chunks [][]*keypair.Full
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

type SendLog struct {
	deviceId      int
	index         int
	server        string
	horizon       *horizonclient.Client
	batchAddress  *keypair.Full
	deviceKeypair *keypair.Full
	account       *horizon.Account
}

func main() {
	keypairs := make([]*keypair.Full, noDevices)
	for i := 0; i < noDevices; i++ {
		keypairs[i] = keypair.MustRandom()
	}
	masterAccount, err := loadMasterAccount()
	if err != nil {
		log.Fatal(err)
	}
	res, err := createAccounts([]*keypair.Full{batchKeypair}, masterKp, masterAccount, randomHorizon())
	if err != nil {
		hError := err.(*horizonclient.Error)
		log.Println("Error submitting transaction:", hError.Problem.Extras["result_codes"])
	}
	if res != nil {
		log.Println(res.Successful)
	}

	chunks := chunkSlice(keypairs, 100)
	for _, chunk := range chunks {
		_, err := createAccounts(chunk, masterKp, masterAccount, randomHorizon())
		if err != nil {
			hError := err.(*horizonclient.Error)
			log.Fatal("Error submitting transaction:", hError.Problem.Extras)
		}
	}

	channels := make([]chan LoadAccountResult, len(keypairs))
	for i := 0; i < len(channels); i++ {
		channels[i] = loadAccountChan(keypairs[i].Address())
	}

	sendLogs := make([]SendLog, len(keypairs))
	for i := 0; i < len(keypairs); i++ {
		result := <-channels[i]
		if result.Error != nil {
			log.Println(result.Error)
			hError := result.Error.(*horizonclient.Error)
			log.Println("Error submitting transaction:", hError.Problem.Extras["result_codes"])
		}
		sendLogs[i] = SendLog{
			deviceId:      i,
			server:        randomStellarCoreUrl(),
			horizon:       randomHorizon(),
			batchAddress:  batchKeypair,
			deviceKeypair: keypairs[i],
			account:       result.Account,
		}
	}

	resultChans := make([]chan SendLogResult, logsNumber*len(keypairs))
	for i := 0; i < logsNumber; i++ {
		for j := 0; j < len(keypairs); j++ {
			log.Println("sleep", time.Duration(1000.0/tps)*time.Millisecond)
			time.Sleep(time.Duration(1000.0/tps) * time.Millisecond)
			params := sendLogs[j]
			log.Printf("Sending log tx %d%d index: %d", i, j, i*len(keypairs)+j)
			resultChans[i*len(keypairs)+j] = sendLogTx(params.deviceId, i, params.server, params.batchAddress, params.deviceKeypair, params.account, params.horizon)
		}
	}

	for i := 0; i < logsNumber*len(keypairs); i++ {
		result := <-resultChans[i]
		if result.Error != nil {
			log.Printf("Error sending log %d %+v\n", i, result.Error)
		} else if result.HTTPResponse != nil {
			defer result.HTTPResponse.Body.Close()
			body, err := ioutil.ReadAll(result.HTTPResponse.Body)
			if err != nil {
				log.Printf("Error reading body of log %d %v", i, err)
			} else {
				log.Printf("Success sending log %d %s", i, string(body))
			}
		} else if result.HorizonResponse != nil {
		}
	}
}
