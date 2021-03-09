package aggregator

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/txnbuild"
	"github.com/stellot/stellot-iot/pkg/crypto"
	"github.com/stellot/stellot-iot/pkg/functions"
	"github.com/stellot/stellot-iot/pkg/helpers"
	"github.com/stellot/stellot-iot/pkg/usecases"
	"github.com/stellot/stellot-iot/pkg/utils"
)

type Aggregator struct {
	Name         string
	TimeInterval TimeInterval
	Blocks       int64
	Keypair      *keypair.Full
}

type TimeInterval uint

const (
	TX TimeInterval = iota

	FIVE_SECS
	THIRTY_SECS

	ONE_MIN
	FIVE_MINS
	THIRTY_MINS

	ONE_HOUR
	SIX_HOURS
	TWELVE_HOURS

	ONE_DAY
)

var Aggregators = []Aggregator{
	// {
	// 	Name:         "tx",
	// 	TimeInterval: TX,
	// 	Blocks:       0,
	// 	Keypair:      nil,
	// },

	// Seconds
	{
		Name:         "5 secs",
		TimeInterval: FIVE_SECS,
		Blocks:       1,
		Keypair:      helpers.FiveSecondsKeypair,
	},
	{
		Name:         "30 secs",
		TimeInterval: THIRTY_SECS,
		Blocks:       6,
		Keypair:      helpers.ThirtySecondsKeypair,
	},

	// Minutes
	{
		Name:         "1 min",
		TimeInterval: ONE_MIN,
		Blocks:       12,
		Keypair:      helpers.OneMinuteKeypair,
	},
	{
		Name:         "5 min",
		TimeInterval: FIVE_MINS,
		Blocks:       60,
		Keypair:      helpers.FiveMinutesKeypair,
	},
	{
		Name:         "30 min",
		TimeInterval: THIRTY_MINS,
		Blocks:       360,
		Keypair:      helpers.ThirtyMinutesKeypair,
	},

	// Hours
	{
		Name:         "1 hours",
		TimeInterval: ONE_HOUR,
		Blocks:       720,
		Keypair:      helpers.OneHourKeypair,
	},
	{
		Name:         "6 hours",
		TimeInterval: SIX_HOURS,
		Blocks:       4320,
		Keypair:      helpers.SixHoursKeypair,
	},
	{
		Name:         "12 hours",
		TimeInterval: TWELVE_HOURS,
		Blocks:       8640,
		Keypair:      helpers.TwelveHoursKeypair,
	},

	// Days
	{
		Name:         "1 day",
		TimeInterval: ONE_DAY,
		Blocks:       17280,
		Keypair:      helpers.OneDayKeypair,
	},
}

func ByTimeInterval(timeInterval TimeInterval) Aggregator {
	for _, v := range Aggregators {
		if v.TimeInterval == timeInterval {
			return v
		}
	}
	panic("Unsupported time interval")
}

func CalculateFunctionsForLedger(dbpool *pgxpool.Pool, sensorAddress string, ledgerSeq int64) (avg int, min int, max int, err error) {
	rows, err := dbpool.Query(context.Background(), "SELECT memo, tx_envelope FROM history_transactions WHERE account = $1 AND ledger_sequence = $2", sensorAddress, ledgerSeq)
	if err != nil {
		log.Fatal(err)
	}
	values := make([]int, 0)

	for rows.Next() {
		var (
			memo       string
			txenvelope string
		)
		err := rows.Scan(&memo, &txenvelope)
		if err != nil {
			log.Fatal(err)
		}
		transaction, err := txnbuild.TransactionFromXDR(txenvelope)
		if err != nil {
			log.Fatal(err)
		}
		tx, ok := transaction.Transaction()
		if !ok {
			log.Fatal("Can not get simple transaction")
		}
		ops := tx.Operations()
		for _, op := range ops {
			val, ok := op.(*txnbuild.Payment)
			if !ok {
				log.Println("Tx is not payment")
			} else {
				if val.Destination == helpers.BatchKeypair.Address() {
					log.Println("Sent to batch address")
					if val.Asset == usecases.TEMP.Asset() || val.Asset == usecases.HUMD.Asset() {
						log.Printf("Sent %s TEMP or HUMD\n", val.Amount)
						value := getLogValue(tx, val)
						values = append(values, value)
						log.Println("values", values)
					}
				}
			}
		}
	}

	if len(values) == 0 {
		return 0, 0, 0, errors.New("no records found")
	}
	var sum int
	if len(values) > 0 {
		min = values[0]
	}
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += v
	}
	avg = sum / len(values)
	return
}

func CalculateFunctionsForLedgers(dbpool *pgxpool.Pool, sensorAddress string, ledgerSeqStart int64, ledgerSeqEnd int64) (avg int, min int, max int, startAccountSeq *int64, endAccountSeq *int64, err error) {
	rows, err := dbpool.Query(context.Background(), "SELECT account_sequence, memo, tx_envelope FROM history_transactions WHERE account = $1 AND ledger_sequence >= $2 AND ledger_sequence < $3", sensorAddress, ledgerSeqStart, ledgerSeqEnd) // should we care about sensor sequences or time index accounts ?
	if err != nil {
		log.Fatal(err)
	}
	values := make([]int, 0)

	for rows.Next() {
		var (
			accountSeq int64
			memo       string
			txenvelope string
		)
		err := rows.Scan(&accountSeq, &memo, &txenvelope)
		if err != nil {
			log.Fatal(err)
		}
		if startAccountSeq == nil {
			startAccountSeq = &accountSeq
		}
		endAccountSeq = &accountSeq

		transaction, err := txnbuild.TransactionFromXDR(txenvelope)
		if err != nil {
			log.Fatal(err)
		}
		tx, ok := transaction.Transaction()
		if !ok {
			log.Fatal("Can not get simple transaction")
		}
		ops := tx.Operations()
		for _, op := range ops {
			val, ok := op.(*txnbuild.Payment)
			if !ok {
				log.Println("Tx is not payment")
			} else {
				if val.Destination == helpers.BatchKeypair.Address() {
					log.Println("Sent to batch address")
					if val.Asset == usecases.TEMP.Asset() || val.Asset == usecases.HUMD.Asset() {
						log.Printf("Sent %s TEMP or HUMD\n", val.Amount)
						value := getLogValue(tx, val)
						values = append(values, value)
					}
				}
			}
		}
	}
	log.Println("aggregating on values", values)

	if len(values) == 0 {
		return 0, 0, 0, nil, nil, errors.New("no records found")
	}
	var sum int
	if len(values) > 0 {
		min = values[0]
	}
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += v
	}
	avg = sum / len(values)
	return
}

func CalculateFunctionsForLastBlocks(dbpool *pgxpool.Pool, timeAddress string, lastBlocks int) (avg int, min int, max int, err error) {
	rows, err := dbpool.Query(context.Background(), "SELECT tx_envelope, ledger_sequence FROM history_transactions WHERE account = $1 ORDER BY account_sequence DESC LIMIT $2", timeAddress, lastBlocks)

	if err != nil {
		log.Fatal(err)
	}
	values := make([]int, 0)

	for rows.Next() {
		var (
			memo       string
			txenvelope string
		)
		err := rows.Scan(&memo, &txenvelope)
		if err != nil {
			log.Fatal(err)
		}
		transaction, err := txnbuild.TransactionFromXDR(txenvelope)
		if err != nil {
			log.Fatal(err)
		}
		tx, ok := transaction.Transaction()
		if !ok {
			log.Fatal("Can not get simple transaction")
		}
		ops := tx.Operations()
		for _, op := range ops {
			val, ok := op.(*txnbuild.Payment)
			if !ok {
				log.Println("Tx is not payment")
			} else {
				if val.Destination == helpers.BatchKeypair.Address() {
					log.Println("Sent to batch address")
					if val.Asset == usecases.TEMP.Asset() || val.Asset == usecases.HUMD.Asset() {
						log.Printf("Sent %s TEMP or HUMD\n", val.Amount)
						value := getLogValue(tx, val)
						values = append(values, value)
					}
				}
			}
		}
	}
	log.Println("aggregating on values", values)

	if len(values) == 0 {
		return 0, 0, 0, errors.New("no records found")
	}
	var sum int
	if len(values) > 0 {
		min = values[0]
	}
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += v
	}
	avg = sum / len(values)
	return
}

func getLogValue(tx *txnbuild.Transaction, op *txnbuild.Payment) int {
	defer utils.Duration(utils.Track("getLogValue"))
	srcAccount := tx.SourceAccount()
	genericMemo, ok := tx.Memo().(txnbuild.MemoHash)
	if !ok {
		log.Fatalln("Can not cast memo to MemoHash")
	}
	memo := txnbuild.MemoHash(genericMemo)
	seqNumber, err := srcAccount.GetSequenceNumber()
	if err != nil {
		log.Fatal(err)
	}
	decrypted, err := crypto.EncryptToMemo(seqNumber, helpers.BatchKeypair, srcAccount.GetAccountID(), memo)
	if err != nil {
		log.Fatal(err)
	}
	decryptedValue := strings.Trim(string(decrypted[:]), string(rune(0)))
	intValue, err := strconv.ParseInt(decryptedValue, 10, 32)
	if err != nil {
		log.Fatal(err)
	}
	return int(intValue)
}

func SendTransaction(sourceAcc *horizon.Account, keypair *keypair.Full, function functions.FunctionType, avg int, sensorAddress string, startAccountSeq int64, endAccountSeq int64) {
	sendAggreateMessage(sourceAcc, keypair, avg, function, sensorAddress, startAccountSeq, endAccountSeq)
}

func sendAggreateMessage(sourceAcc *horizon.Account, keypair *keypair.Full, value int, functionType functions.FunctionType, sensorAddress string, startLedger int64, endLedger int64) {
	seqNumber, err := sourceAcc.GetSequenceNumber()
	if err != nil {
		log.Fatal(err)
	}
	var payload [32]byte
	copy(payload[:], strconv.Itoa(value))
	cipher, err := crypto.EncryptToMemo(seqNumber+1, keypair, sensorAddress, payload)
	memo := txnbuild.MemoHash(*cipher)

	ops := []txnbuild.Operation{
		&txnbuild.BumpSequence{
			BumpTo: startLedger,
		},
		&txnbuild.BumpSequence{
			BumpTo: endLedger,
		},
		&txnbuild.Payment{
			Asset:       functionType.Asset(),
			Amount:      "1",
			Destination: sensorAddress,
		}}
	helpers.MustSendTransaction(sourceAcc, keypair, ops, memo)
}
