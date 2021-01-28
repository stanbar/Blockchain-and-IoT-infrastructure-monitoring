package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stellar/go/keypair"
	"github.com/stellot/stellot-iot/pkg/crypto"
	"github.com/stellot/stellot-iot/pkg/functions"
	"github.com/stellot/stellot-iot/pkg/helpers"
)

func main() {
	dbpool, err := pgxpool.Connect(context.Background(), helpers.DatabaseUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}

	defer dbpool.Close()
	timeKeypairs := helpers.TimeIndexAccounts

	for i, timeKeypair := range timeKeypairs {
		avgs := getFromLastNFiveSecondsIntervals(dbpool, functions.AvgAssetName, timeKeypair, helpers.DevicesKeypairs[0].Address(), 5)
		mins := getFromLastNFiveSecondsIntervals(dbpool, functions.MinAssetName, timeKeypair, helpers.DevicesKeypairs[0].Address(), 5)
		maxs := getFromLastNFiveSecondsIntervals(dbpool, functions.MaxAssetName, timeKeypair, helpers.DevicesKeypairs[0].Address(), 5)
		log.Println(i, avgs, mins, maxs)
	}
}

func getFromLastNFiveSecondsIntervals(dbpool *pgxpool.Pool, function string, timeKeypair *keypair.Full, sensorAddres string, lastBlocks int64) []int {

	rows, err := dbpool.Query(context.Background(), `SELECT memo, account_sequence FROM history_operations ops
JOIN history_transactions txs on ops.transaction_id = txs.id
WHERE type = 1
	AND details->>'from' = $1
	AND details->>'to' = $2
	AND details->>'asset_code' = $3
ORDER BY account_sequence DESC
LIMIT $4;`, timeKeypair.Address(), sensorAddres, function, lastBlocks)

	if err != nil {
		log.Fatal(err)
	}
	values := make([]int, 0)

	for rows.Next() {
		var (
			memo       string
			accountSeq int64
		)
		err := rows.Scan(&memo, &accountSeq)
		if err != nil {
			log.Fatal(err)
		}

		out := make([]byte, base64.StdEncoding.DecodedLen(len(memo)))
		length, err := base64.StdEncoding.Decode(out, []byte(memo))
		if err != nil {
			log.Fatal(err)
		}

		var memoBytes [32]byte
		copy(memoBytes[:], out[:length])

		decrypted, err := crypto.EncryptToMemo(accountSeq, timeKeypair, sensorAddres, memoBytes)
		decryptedValue := strings.Trim(string(decrypted[:]), string(rune(0)))
		intValue, err := strconv.ParseInt(decryptedValue, 10, 32)
		values = append(values, int(intValue))
	}
	return values
}
