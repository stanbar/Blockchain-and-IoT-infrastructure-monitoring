package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/protocols/horizon"
	"github.com/stellot/stellot-iot/pkg/aggregator"
	"github.com/stellot/stellot-iot/pkg/functions"
	"github.com/stellot/stellot-iot/pkg/helpers"
	"github.com/stellot/stellot-iot/pkg/utils"
)

func main() {
	http.DefaultClient.Timeout = time.Second * time.Duration(helpers.TimeOut)
	rand.Seed(time.Now().UnixNano())
	helpers.BlockUntilHorizonIsReady()
	masterAcc := helpers.MustLoadMasterAccount()
	sensorKeypairs := helpers.DevicesKeypairs

	aggregatorKeypairs := make([]*keypair.Full, len(aggregator.Aggregators))
	for i, v := range aggregator.Aggregators {
		aggregatorKeypairs[i] = v.Keypair
	}

	helpers.MustCreateAccounts(masterAcc, aggregatorKeypairs, "horizon")
	helpers.MustCreateAccounts(masterAcc, []*keypair.Full{functions.AssetKeypair}, "horizon")

	log.Println("Loading aggregation accounts")
	aggregatorAccounts := loadAccounts(masterAcc, aggregatorKeypairs)

	log.Println("Loading sensor accounts")
	sensorAccounts := loadAccounts(masterAcc, sensorKeypairs)

	log.Println("Loading functions asset account")
	assetAccount := helpers.MustLoadAccount(functions.AssetKeypair.Address())

	createTrustlines(masterAcc, aggregatorKeypairs, aggregatorAccounts, "horizon")
	createTrustlines(masterAcc, sensorKeypairs, sensorAccounts, "horizon")

	for _, v := range functions.Assets {
		helpers.MustFundAccountsEvenly(masterAcc, assetAccount, functions.AssetKeypair, aggregatorKeypairs, v)
	}

	// log.Println("Loaded tmie accounts", accounts)

	dbpool, err := pgxpool.Connect(context.Background(), helpers.DatabaseUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}

	defer dbpool.Close()

	for _, sensorKp := range sensorKeypairs {
		log.Println("Aggregating for sensor: ", sensorKp.Address())
		for i, v := range aggregator.Aggregators {
			log.Println("Aggregating on: ", v.Name)
			aggregateForNBlocksInterval(dbpool, v.Blocks, sensorKp, aggregatorAccounts[i], v.Keypair)
		}
	}
}

type TimeIndex struct {
	keypair *keypair.Full
	account *horizon.Account
}

func (t TimeIndex) Keypair() *keypair.Full {
	return t.keypair
}

func (t TimeIndex) Account() *horizon.Account {
	return t.account
}

func loadAccounts(masterAcc *horizon.Account, keypairs []*keypair.Full) []*horizon.Account {
	channels := make([]chan helpers.LoadAccountResult, len(keypairs))
	for i := 0; i < len(channels); i++ {
		channels[i] = helpers.LoadAccountChan(keypairs[i].Address())
	}
	accounts := make([]*horizon.Account, len(keypairs))
	for i := 0; i < len(keypairs); i++ {
		result := <-channels[i]
		if result.Error != nil {
			log.Fatal(result.Error)
		}
		accounts[i] = result.Account
	}
	return accounts
}

func createTrustlines(masterAcc *horizon.Account, keypairs []*keypair.Full, accounts []*horizon.Account, where string) {
	timeIndexes := make([]helpers.CreateTrustline, len(keypairs))
	for i, v := range keypairs {
		timeIndexes[i] = TimeIndex{
			keypair: v,
			account: accounts[i],
		}
	}

	helpers.MustCreateTrustlines(masterAcc, timeIndexes, functions.Assets, where)
}

func aggregateForNBlocksInterval(dbpool *pgxpool.Pool, blocks int64, sensor *keypair.Full, timeAccount *horizon.Account, timeKeypair *keypair.Full) {
	defer utils.Duration(utils.Track("aggregateForNBlocksInterval"))

	row := dbpool.QueryRow(context.Background(), "SELECT ledger_sequence, account_sequence FROM history_transactions WHERE account = $1 ORDER BY ledger_sequence ASC LIMIT 1", sensor.Address())
	var (
		firstLedgerSeq  int64
		firstAccountSeq int64
	)
	err := row.Scan(&firstLedgerSeq, &firstAccountSeq)
	if err != nil {
		log.Fatal("Did not find first row")
	}

	row = dbpool.QueryRow(context.Background(), "SELECT ledger_sequence, account_sequence FROM history_transactions WHERE account = $1 ORDER BY ledger_sequence DESC LIMIT 1", sensor.Address())
	var (
		lastLedgerSeq  int64
		lastAccountSeq int64
	)
	row.Scan(&lastLedgerSeq, &lastAccountSeq)
	if err != nil {
		log.Fatal("Did not find last row")
	}

	for currentLedger := firstLedgerSeq; currentLedger < lastLedgerSeq; currentLedger += blocks {
		avg, min, max, startAccSeq, endAccSeq, err := aggregator.CalculateFunctionsForLedgers(dbpool, sensor.Address(), currentLedger, currentLedger+blocks) // what ledgerSeq to use ?
		if err != nil {
			log.Println(err)
			continue
		}
		log.Printf("avg: %d min: %d max: %d\n", avg, min, max)
		time.Sleep(1000 / 50 / 3 * time.Millisecond)
		aggregator.SendTransaction(timeAccount, timeKeypair, functions.AVG, avg, sensor.Address(), *startAccSeq, *endAccSeq)
		time.Sleep(1000 / 50 / 3 * time.Millisecond)
		aggregator.SendTransaction(timeAccount, timeKeypair, functions.MIN, min, sensor.Address(), *startAccSeq, *endAccSeq)
		time.Sleep(1000 / 50 / 3 * time.Millisecond)
		aggregator.SendTransaction(timeAccount, timeKeypair, functions.MAX, max, sensor.Address(), *startAccSeq, *endAccSeq)
	}
	log.Printf("Finished aggregating %d blocks from %d to %d\n", blocks, firstLedgerSeq, lastLedgerSeq)
}
