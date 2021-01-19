package helpers

import (
	"log"
	"math/rand"
	"strconv"
	"strings"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/txnbuild"
	"github.com/stellot/stellot-iot/pkg/utils"
)

var (
	NetworkPassphrase      = utils.MustGetenv("NETWORK_PASSPHRASE")
	DatabaseUrl            = utils.MustGetenv("DATABASE_URL")
	stellarCoreUrls        = utils.MustGetenv("STELLAR_CORE_URLS")
	stellarCoreUrlsSlice   = strings.Split(stellarCoreUrls, " ")
	devicesSecrets         = utils.MustGetenv("DEVICES_SECRETS")
	devicesSecretsSlice    = strings.Split(devicesSecrets, " ")
	DevicesKeypairs        = devicesKeypairs()
	horizonServerUrls      = utils.MustGetenv("HORIZON_SERVER_URLS")
	horizonServerUrlsSlice = strings.Split(horizonServerUrls, " ")
	horizonServers         = CreateHorizonServers()
	MasterKp, _            = keypair.FromRawSeed(network.ID(NetworkPassphrase))
	LogsNumber, _          = strconv.Atoi(utils.MustGetenv("LOGS_NUMBER"))
	Peroid, _              = strconv.Atoi(utils.MustGetenv("PEROID"))
	SendTxTo               = utils.MustGetenv("SEND_TX_TO")
	Tps, _                 = strconv.Atoi(utils.MustGetenv("TPS"))
	TimeOut, _             = strconv.ParseInt(utils.MustGetenv("SEND_TO_CORE_TIMEOUT_SECONDS"), 10, 64)
	BatchKeypair           = keypair.MustParseFull(utils.MustGetenv("BATCH_SECRET_KEY"))
	FiveSecondsKeypair     = keypair.MustParseFull(utils.MustGetenv("FIVE_SECONDS_SECRET"))
	TenSecondsKeypair      = keypair.MustParseFull(utils.MustGetenv("TEN_SECONDS_SECRET"))
	ThirtySecondsKeypair   = keypair.MustParseFull(utils.MustGetenv("THIRTY_SECONDS_SECRET"))
	OneMinuteKeypair       = keypair.MustParseFull(utils.MustGetenv("ONE_MINUTE_SECRET"))
	TimeIndexAccounts      = []*keypair.Full{FiveSecondsKeypair, TenSecondsKeypair, ThirtySecondsKeypair, OneMinuteKeypair}
)

func devicesKeypairs() []*keypair.Full {
	keypairs := make([]*keypair.Full, len(devicesSecretsSlice))
	for i, v := range devicesSecretsSlice {
		key, err := keypair.ParseFull(v)
		if err != nil {
			panic(err)
		}
		keypairs[i] = key
	}
	return keypairs
}

func CreateHorizonServers() []*horizonclient.Client {
	horizons := make([]*horizonclient.Client, len(horizonServerUrlsSlice))
	for i, v := range horizonServerUrlsSlice {
		horizons[i] = &horizonclient.Client{
			HorizonURL: v,
		}
	}
	return horizons
}

func RandomHorizon() *horizonclient.Client {
	return horizonServers[rand.Intn(len(horizonServers))]
}

func RandomStellarCoreUrl() string {
	return stellarCoreUrlsSlice[rand.Intn(len(stellarCoreUrlsSlice))]
}

func LoadMasterAccount() (*horizon.Account, error) {
	return LoadAccount(MasterKp.Address())
}

func LoadAccount(accountId string) (*horizon.Account, error) {
	accReq := horizonclient.AccountRequest{AccountID: accountId}
	masterAccount, err := RandomHorizon().AccountDetail(accReq)
	if err != nil {
		return nil, err
	}
	return &masterAccount, nil
}

func HandleGracefuly(err error) {
	if err != nil {
		hError, ok := err.(*horizonclient.Error)
		if ok {
			if hError.Problem.Extras["result_codes"] != nil {
				log.Printf("Error submitting tx result_codes: %s\n", hError.Problem.Extras["result_codes"])
			} else if hError.Problem.Extras["envelope_xdr"] != nil {
				log.Printf("Error submitting tx envelope_xdr: %s\n", hError.Problem.Extras["envelope_xdr"])
			} else if hError != nil {
				log.Printf("Error submitting tx: %v %s\n", hError, hError.Problem)
			}
		} else {
			log.Printf("Error submitting tx: %s\n", err)
		}
	} else {
		log.Println("Successfully submitted tx")
	}
}

func CreateAccounts(kp []*keypair.Full, client *horizonclient.Client) error {
	masterAccount, err := LoadMasterAccount()
	if err != nil {
		log.Fatal(err)
	}
	createAccountOps := make([]txnbuild.Operation, len(kp))

	for i, v := range kp {
		createAccountOps[i] = &txnbuild.CreateAccount{
			Destination: v.Address(),
			Amount:      "100",
		}
	}
	txParams := txnbuild.TransactionParams{
		SourceAccount:        masterAccount,
		IncrementSequenceNum: true,
		Operations:           createAccountOps,
		Timebounds:           txnbuild.NewTimeout(120),
		BaseFee:              100,
	}

	tx, err := txnbuild.NewTransaction(txParams)
	if err != nil {
		return err
	}
	signedTx, err := tx.Sign(NetworkPassphrase, MasterKp)
	if err != nil {
		return err
	}
	log.Println("Submitting createAccount transaction")
	_, err = client.SubmitTransactionWithOptions(signedTx, horizonclient.SubmitTxOpts{SkipMemoRequiredCheck: true})
	return err
}

type CreateTrustline interface {
	Keypair() *keypair.Full
	Account() *horizon.Account
}

func CreateTrustlines(accounts []CreateTrustline, assets []txnbuild.Asset) error {
	masterAcc, err := LoadMasterAccount()
	if err != nil {
		return err
	}
	chunks := chunk(accounts, 19) // Stellar allows up to 20 signatures, and 1 is reserved to master
	for _, chunk := range chunks {
		fundAccountsOps := make([]txnbuild.Operation, len(chunk))
		for i, account := range chunk {
			for j, asset := range assets {
				fundAccountsOps[i+i*j] = &txnbuild.ChangeTrust{
					Line:          asset,
					SourceAccount: account.Account(),
				}
			}
		}
		txParams := txnbuild.TransactionParams{
			SourceAccount:        masterAcc,
			IncrementSequenceNum: true,
			Operations:           fundAccountsOps,
			Timebounds:           txnbuild.NewTimeout(120),
			BaseFee:              100,
		}

		tx, err := txnbuild.NewTransaction(txParams)
		if err != nil {
			return err
		}

		signers := make([]*keypair.Full, len(chunk)+1)
		signers[0] = MasterKp
		for i, v := range chunk {
			signers[i+1] = v.Keypair()
		}

		signedTx, err := tx.Sign(NetworkPassphrase, signers...)
		if err != nil {
			return err
		}
		log.Println("Submitting createTrustlines transaction")
		_, err = RandomHorizon().SubmitTransactionWithOptions(signedTx, horizonclient.SubmitTxOpts{SkipMemoRequiredCheck: true})
		HandleGracefuly(err)
	}
	return nil
}

func chunk(slice []CreateTrustline, chunkSize int) [][]CreateTrustline {
	var chunks [][]CreateTrustline
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

type LoadAccountResult struct {
	Account *horizon.Account
	Error   error
}

func LoadAccountChan(accountId string) chan LoadAccountResult {
	ch := make(chan LoadAccountResult)
	accReq := horizonclient.AccountRequest{AccountID: accountId}
	go func() {
		masterAccount, err := RandomHorizon().AccountDetail(accReq)
		if err != nil {
			ch <- LoadAccountResult{Account: nil, Error: err}
		} else {
			ch <- LoadAccountResult{Account: &masterAccount, Error: nil}
		}
	}()
	return ch
}
