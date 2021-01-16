package helpers

import (
	"math/rand"
	"strconv"
	"strings"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/protocols/horizon"
	"github.com/stellot/stellot-iot/pkg/utils"
)

var NetworkPassphrase = utils.MustGetenv("NETWORK_PASSPHRASE")
var stellarCoreUrls = utils.MustGetenv("STELLAR_CORE_URLS")
var stellarCoreUrlsSlice = strings.Split(stellarCoreUrls, " ")
var devicesSecrets = utils.MustGetenv("DEVICES_SECRETS")
var devicesSecretsSlice = strings.Split(devicesSecrets, " ")
var horizonServerUrls = utils.MustGetenv("HORIZON_SERVER_URLS")
var horizonServerUrlsSlice = strings.Split(horizonServerUrls, " ")
var horizonServers = CreateHorizonServers()
var MasterKp, _ = keypair.FromRawSeed(network.ID(NetworkPassphrase))
var LogsNumber, _ = strconv.Atoi(utils.MustGetenv("LOGS_NUMBER"))
var Peroid, _ = strconv.Atoi(utils.MustGetenv("PEROID"))
var SendTxTo = utils.MustGetenv("SEND_TX_TO")
var Tps, _ = strconv.Atoi(utils.MustGetenv("TPS"))
var TimeOut, _ = strconv.ParseInt(utils.MustGetenv("SEND_TO_CORE_TIMEOUT_SECONDS"), 10, 64)
var BatchKeypair = keypair.MustParseFull(utils.MustGetenv("BATCH_SECRET_KEY"))

func DevicesKeypairs() []*keypair.Full {
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
	return loadAccount(MasterKp.Address())
}

func loadAccount(accountId string) (*horizon.Account, error) {
	accReq := horizonclient.AccountRequest{AccountID: accountId}
	masterAccount, err := RandomHorizon().AccountDetail(accReq)
	if err != nil {
		return nil, err
	}
	return &masterAccount, nil
}
