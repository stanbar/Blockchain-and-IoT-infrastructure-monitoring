package aggregator

import (
	"context"
	"log"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stellar/go/txnbuild"
	"github.com/stellot/stellot-iot/pkg/crypto"
	"github.com/stellot/stellot-iot/pkg/helpers"
	"github.com/stellot/stellot-iot/pkg/usecases"
)

func CalculateFunctionsForLedger(dbpool *pgxpool.Pool, sensorAddress string, ledgerSeq int) (avg int, min int, max int) {
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
	log.Println("decrypted memo:", string(decrypted[:]))
	decryptedValue := strings.Trim(string(decrypted[:]), string(rune(0)))
	intValue, err := strconv.ParseInt(decryptedValue, 10, 32)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("intValue", intValue)
	return int(intValue)
}
