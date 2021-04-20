package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stellar/go/keypair"
	"github.com/stellot/stellot-iot/pkg/aggregator"
	"github.com/stellot/stellot-iot/pkg/crypto"
	"github.com/stellot/stellot-iot/pkg/functions"
	"github.com/stellot/stellot-iot/pkg/helpers"
	"github.com/stellot/stellot-iot/pkg/usecases"
	"github.com/stellot/stellot-iot/pkg/utils"
)

func main() {
	dbpool, err := pgxpool.Connect(context.Background(), helpers.DatabaseUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}

	defer dbpool.Close()

	// for _, aggregator := range aggregator.Aggregators[3:] {
	// 	res := getValuesFromPeroid(dbpool, functions.AvgAssetName, helpers.DevicesKeypairs[0].Address(), aggregator, 10000000000)
	// 	log.Println("avg", res)
	// }

	// for _, aggregator := range aggregator.Aggregators[3:] {
	// 	res := getValuesFromPeroid(dbpool, functions.MinAssetName, helpers.DevicesKeypairs[0].Address(), aggregator, 10000000000)
	// 	log.Println("min", res)
	// }

	// for _, aggregator := range aggregator.Aggregators[3:] {
	// 	res := getValuesFromPeroid(dbpool, functions.MaxAssetName, helpers.DevicesKeypairs[0].Address(), aggregator, 10000000000)
	// 	log.Println("max", res)
	// }

	// for _, aggregator := range aggregator.Aggregators {
	// 	res := getValuesPredicate(dbpool, functions.MinAssetName, helpers.DevicesKeypairs[0].Address(), aggregator, ">", 500)
	// 	log.Println("min > 500", res)
	// }

	// for _, aggregator := range aggregator.Aggregators {
	// 	res := getValuesPredicate(dbpool, functions.MaxAssetName, helpers.DevicesKeypairs[0].Address(), aggregator, "<", 700)
	// 	log.Println("max < 700", res)
	// }

	// getValuesPredicateTx(dbpool, usecases.HUMD, helpers.DevicesKeypairs[0], "<", 700)

	// countTxs(dbpool, helpers.DevicesKeypairs[0].Address(), usecases.TEMP, functions.AVG, 2, aggregator.SIX_HOURS)
	// countTxs(dbpool, helpers.DevicesKeypairs[0].Address(), usecases.HUMD, functions.AVG, 1, aggregator.FIVE_SECS)
	// countTxs(dbpool, helpers.DevicesKeypairs[0].Address(), usecases.HUMD, functions.AVG, 1, aggregator.THIRTY_SECS)
	// countTxs(dbpool, helpers.DevicesKeypairs[0].Address(), usecases.HUMD, functions.AVG, 1, aggregator.ONE_MIN)
	// countTxs(dbpool, helpers.DevicesKeypairs[0].Address(), usecases.HUMD, functions.AVG, 1, aggregator.FIVE_MINS)
	// countTxs(dbpool, helpers.DevicesKeypairs[0].Address(), usecases.TEMP, functions.AVG, 1, aggregator.SIX_HOURS)
	// countTxsTimeBounds(dbpool, helpers.DevicesKeypairs[0].Address(), 1, aggregator.SIX_HOURS)

	// values := getValuesPredicate(dbpool, functions.AVG, helpers.DevicesKeypairs[0].Address(), aggregator.ByTimeInterval(aggregator.FIVE_SECS), "<", 700)
	// log.Println(len(values))
	// values = getValuesPredicateTx(dbpool, usecases.TEMP, helpers.DevicesKeypairs[0], "<", 700)
	// log.Println(len(values))
	// values := getValuesFromPeroidTx(dbpool, helpers.DevicesKeypairs[0].Address(), 100000000000)
	// log.Println(len(values))
	// values = getValuesFromLastNIntervals(dbpool, functions.AVG, helpers.DevicesKeypairs[0].Address(), aggregator.ByTimeInterval(aggregator.FIVE_SECS), 100000000000)
	// log.Println(len(values))
	// values = getValuesFromPeroid(dbpool, functions.AVG, helpers.DevicesKeypairs[0].Address(), aggregator.ByTimeInterval(aggregator.ONE_MIN), 100000000000)
	// log.Println(len(values))
	// values = getValuesFromPeroid(dbpool, functions.AVG, helpers.DevicesKeypairs[0].Address(), aggregator.ByTimeInterval(aggregator.ONE_HOUR), 100000000000)
	// log.Println(len(values))
	// values = getValuesFromPeroid(dbpool, functions.AVG, helpers.DevicesKeypairs[0].Address(), aggregator.ByTimeInterval(aggregator.ONE_DAY), 100000000000)
	// log.Println(len(values))
	// values = queryFiveAggregated(dbpool, functions.AVG, helpers.DevicesKeypairs[0].Address(), aggregator.ByTimeInterval(aggregator.FIVE_SECS), 100000000000)
	value := get3Tx(dbpool, helpers.DevicesKeypairs[0].Address())
	log.Println(value)
	value64 := get3Agg(dbpool, helpers.DevicesKeypairs[0].Address(), functions.AVG, aggregator.SIX_HOURS, 1)
	log.Println("six hours 1", value64)
	value64 = get3Agg(dbpool, helpers.DevicesKeypairs[0].Address(), functions.AVG, aggregator.SIX_HOURS, 2)
	log.Println("six hours 2", value64)
	value64 = get3Agg(dbpool, helpers.DevicesKeypairs[0].Address(), functions.AVG, aggregator.FIVE_MINS, 1)
	log.Println("one hour 1", value64)

}

func countTxs(dbpool *pgxpool.Pool, sensorAddress string, usecase usecases.PhysicsType, function functions.FunctionType, lastTxs int, timeInterval aggregator.TimeInterval) {
	defer utils.Duration(utils.Track("countTxs"))
	start := time.Now()

	from := aggregator.ByTimeInterval(timeInterval).Keypair.Address()
	to := sensorAddress
	assetCode := function.Asset().GetCode()
	offset := lastTxs

	log.Println("from: ", from, " to: ", to, " assetCode: ", assetCode, " offset: ", offset)

	row := dbpool.QueryRow(context.Background(), `
  SELECT transaction_id FROM history_operations ops
  JOIN history_transactions txs on ops.transaction_id = txs.id
  WHERE source_account = $1
    AND type = 1
    AND details->>'from' = $2
    AND details->>'to' = $3
    AND details->>'asset_code' = $4
  ORDER BY account_sequence DESC
  LIMIT 1
  OFFSET $5;
  `, from, from, to, assetCode, offset)
	log.Println("executed sql", time.Since(start))

	var txId int64
	err := row.Scan(&txId)
	if err != nil {
		panic(err)
	}
	log.Println("txId: ", txId)

	rows, err := dbpool.Query(context.Background(), `
  SELECT details->>'bump_to' FROM history_operations ops
  WHERE transaction_id = $1
    AND details->>'bump_to' IS NOT NULL
  ORDER BY application_order
  `, txId)
	log.Println("executed sql", time.Since(start))

	var (
		fromSeq string
		toSeq   string
	)
	rows.Next()
	err = rows.Scan(&fromSeq)
	if err != nil {
		panic(err)
	}

	rows.Next()
	err = rows.Scan(&toSeq)
	if err != nil {
		panic(err)
	}

	log.Printf("fromSeq: %s toSeq: %s", fromSeq, toSeq)
	fromInt, err := strconv.ParseUint(fromSeq, 10, 64)
	toInt, err := strconv.ParseUint(toSeq, 10, 64)
	log.Println("Pre interpreted count: ", toInt-fromInt)
}

func countTxsTimeBounds(dbpool *pgxpool.Pool, sensorAddress string, lastTxs int, timeInterval aggregator.TimeInterval) {
	defer utils.Duration(utils.Track("countTxsTimeBounds"))
	start := time.Now()
	to := sensorAddress

	sixHoursAgoUnix := time.Date(2021, 3, 24, 16, 0, 0, 0, time.UTC).Unix() - 100000
	row := dbpool.QueryRow(context.Background(), `
	SELECT count(*) FROM history_transactions txs
	WHERE txs.account = $1
  AND lower(txs.time_bounds) > $2
	GROUP BY txs.account;
	`, to, sixHoursAgoUnix)
	log.Println("executed sql", time.Since(start))

	var count int64
	log.Println("scanning")
	err := row.Scan(&count)
	log.Println("after scanning")
	if err != nil {
		log.Println("Error")
		log.Fatal(err)
		panic(err)
	}

	log.Println("count: ", count)
}

func getValuesFromLastNIntervals(dbpool *pgxpool.Pool, function functions.FunctionType, sensorAddress string, aggregator aggregator.Aggregator, lastTxs int) []int {
	defer utils.Duration(utils.Track("getValuesFromPeroid"))
	start := time.Now()
	rows, err := dbpool.Query(context.Background(), `
  SELECT memo, account_sequence FROM history_operations ops
  JOIN history_transactions txs on ops.transaction_id = txs.id
  WHERE type = 1
    AND details->>'from' = $1
    AND details->>'to' = $2
    AND details->>'asset_code' = $3
  ORDER BY account_sequence DESC
  LIMIT $4;
  `, aggregator.Keypair.Address(), sensorAddress, function.Asset().GetCode(), lastTxs)

	elapsed := time.Since(start)
	log.Println("execute sql", elapsed)

	if err != nil {
		panic(err)
	}
	return parseValues(rows, aggregator.Keypair, sensorAddress)
}

func getValuesFromPeroidTx(dbpool *pgxpool.Pool, sensorAddress string, lastTxs int) []int {
	defer utils.Duration(utils.Track("getValuesFromPeroidTx"))
	start := time.Now()
	rows, err := dbpool.Query(context.Background(), `
  SELECT memo, account_sequence FROM history_operations ops
  JOIN history_transactions txs on ops.transaction_id = txs.id
  WHERE type = 1
    AND details->>'from' = $1
    AND details->>'to' = $2
  ORDER BY account_sequence DESC
  LIMIT $3;
  `, sensorAddress, helpers.BatchKeypair.Address(), lastTxs)

	elapsed := time.Since(start)
	log.Println("executed sql", elapsed)

	if err != nil {
		panic(err)
	}
	return parseValues(rows, helpers.BatchKeypair, sensorAddress)
}

func getValuesPredicateTx(dbpool *pgxpool.Pool, usecase usecases.PhysicsType, sensor *keypair.Full, operation string, predicate int) []int {
	defer utils.Duration(utils.Track("getValuesPredicateTx"))
	from := sensor.Address()
	to := helpers.BatchKeypair.Address()
	assetCode := usecase.Asset().GetCode()

	log.Println("from: ", from, " to: ", to, " assetCode: ", assetCode)

	start := time.Now()
	rows, err := dbpool.Query(context.Background(), `
  SELECT memo, account_sequence FROM history_operations ops
  JOIN history_transactions txs on ops.transaction_id = txs.id
  WHERE type = 1
    AND details->>'from' = $1
    AND details->>'to' = $2
    AND details->>'asset_code' = $3
  ORDER BY account_sequence DESC
  `, from, to, assetCode)
	log.Println("execute sql", time.Since(start))

	if err != nil {
		log.Fatalln(err)
		panic(err)
	}

	start = time.Now()
	values := []int{}
	for _, v := range parseValues(rows, sensor, to) {
		switch operation {
		case ">":
			if v > predicate {
				values = append(values, v)
			}
		case ">=":
			if v >= predicate {
				values = append(values, v)
			}
		case "<":
			if v < predicate {
				values = append(values, v)
			}
		case "<=":
			if v <= predicate {
				values = append(values, v)
			}
		}
	}
	log.Printf("parsed %d values in %s", len(values), time.Since(start))
	return values
}

func getValuesPredicate(dbpool *pgxpool.Pool, function functions.FunctionType, sensorAddress string, aggregator aggregator.Aggregator, operation string, predicate int) []int {
	defer utils.Duration(utils.Track("getValuesPredicate"))
	start := time.Now()
	rows, err := dbpool.Query(context.Background(), `
  SELECT memo, account_sequence FROM history_operations ops
  JOIN history_transactions txs on ops.transaction_id = txs.id
  WHERE type = 1
    AND details->>'from' = $1
    AND details->>'to' = $2
    AND details->>'asset_code' = $3
  ORDER BY account_sequence DESC
  `, aggregator.Keypair.Address(), sensorAddress, function.Asset().GetCode())

	log.Println("execute sql", time.Since(start))

	if err != nil {
		log.Fatal(err)
		panic(err)
	}

	values := []int{}
	for _, v := range parseValues(rows, aggregator.Keypair, sensorAddress) {
		switch operation {
		case ">":
			if v > predicate {
				values = append(values, v)
			}
		case ">=":
			if v >= predicate {
				values = append(values, v)
			}
		case "<":
			if v < predicate {
				values = append(values, v)
			}
		case "<=":
			if v <= predicate {
				values = append(values, v)
			}
		}
	}
	return values
}

func get1agg(dbpool *pgxpool.Pool, sensorAddress string, usecase usecases.PhysicsType, function functions.FunctionType, lastTxs int, timeInterval aggregator.TimeInterval) {
	defer utils.Duration(utils.Track("countTxs"))
	start := time.Now()

	from := aggregator.ByTimeInterval(timeInterval).Keypair.Address()
	to := sensorAddress
	assetCode := function.Asset().GetCode()
	offset := lastTxs

	log.Println("from: ", from, " to: ", to, " assetCode: ", assetCode, " offset: ", offset)

	row := dbpool.QueryRow(context.Background(), `
  SELECT transaction_id FROM history_operations ops
  JOIN history_transactions txs on ops.transaction_id = txs.id
  WHERE source_account = $1
    AND type = 1
    AND details->>'from' = $2
    AND details->>'to' = $3
    AND details->>'asset_code' = $4
  ORDER BY account_sequence DESC
  LIMIT 1
  OFFSET $5;
  `, from, from, to, assetCode, offset)
	log.Println("executed sql", time.Since(start))

	var txId int64
	err := row.Scan(&txId)
	if err != nil {
		panic(err)
	}
	log.Println("txId: ", txId)

	rows, err := dbpool.Query(context.Background(), `
  SELECT details->>'bump_to' FROM history_operations ops
  WHERE transaction_id = $1
    AND details->>'bump_to' IS NOT NULL
  ORDER BY application_order
  `, txId)
	log.Println("executed sql", time.Since(start))

	var (
		fromSeq string
		toSeq   string
	)
	rows.Next()
	err = rows.Scan(&fromSeq)
	if err != nil {
		panic(err)
	}

	rows.Next()
	err = rows.Scan(&toSeq)
	if err != nil {
		panic(err)
	}

	log.Printf("fromSeq: %s toSeq: %s", fromSeq, toSeq)
	fromInt, err := strconv.ParseUint(fromSeq, 10, 64)
	toInt, err := strconv.ParseUint(toSeq, 10, 64)
	log.Println("Pre interpreted count: ", toInt-fromInt)
}

func get3Tx(dbpool *pgxpool.Pool, sensorAddress string) int {
	defer utils.Duration(utils.Track("get3Tx"))
	start := time.Now()

	// sixHoursAgoUnix := time.Date(2021, 3, 24, 16, 0, 0, 0, time.UTC).Unix() - 100000
	sixHoursAgoUnix := time.Date(2021, 3, 24, 16, 0, 0, 0, time.UTC).Unix() - 100000
	rows, err := dbpool.Query(context.Background(), `
	SELECT memo, account_sequence FROM history_transactions txs
	WHERE txs.account = $1
    AND lower(txs.time_bounds) > $2;
	`, sensorAddress, sixHoursAgoUnix)

	if err != nil {
		log.Fatal(err)
		panic(err)
	}

	elapsed := time.Since(start)
	log.Println("executed sql", elapsed)

	values := parseValues(rows, helpers.BatchKeypair, sensorAddress)
	log.Println("read count", len(values))
	sum := 0
	for _, v := range values {
		sum += v
	}
	return sum / len(values)
}

func get3Agg(dbpool *pgxpool.Pool, sensorAddress string, function functions.FunctionType, timeInterval aggregator.TimeInterval, lastBlocks int) int64 {
	defer utils.Duration(utils.Track("get3Agg"))
	start := time.Now()
	keypair := aggregator.ByTimeInterval(timeInterval).Keypair
	row := dbpool.QueryRow(context.Background(), `
  SELECT memo, account_sequence, ransaction_id FROM history_operations ops
  JOIN history_transactions txs on ops.transaction_id = txs.id
  WHERE type = 1
    AND details->>'from' = $1
    AND details->>'to' = $2
    AND details->>'asset_code' = $3
  ORDER BY account_sequence DESC
  OFFSET $4
  LIMIT 1
  `, keypair.Address(), sensorAddress, function.Asset().GetCode(), lastBlocks)
	elapsed := time.Since(start)
	log.Println("executed sql", elapsed)

	privKey := crypto.StellarKeypairToPrivKey(keypair)
	pubKey := crypto.StellarAddressToPubKey(sensorAddress)

	var (
		memo       string
		accountSeq int64
		txId       int64
	)
	err := row.Scan(&memo, &accountSeq, &txId)
	if err != nil {
		log.Fatal(err)
	}
	intValue := parseMemo(memo, accountSeq, privKey, pubKey)
	elapsed = time.Since(start)
	log.Println("parsed memo", elapsed)

	rows, err := dbpool.Query(context.Background(), `
  SELECT details->>'bump_to' FROM history_operations ops
  WHERE transaction_id = $1
    AND details->>'bump_to' IS NOT NULL
  ORDER BY application_order
  `, txId)

	var (
		fromSeq string
		toSeq   string
	)
	rows.Next()
	err = rows.Scan(&fromSeq)
	if err != nil {
		panic(err)
	}

	rows.Next()
	err = rows.Scan(&toSeq)
	if err != nil {
		panic(err)
	}

	fromInt, _ := strconv.ParseInt(fromSeq, 10, 64)
	toInt, _ := strconv.ParseInt(toSeq, 10, 64)
	log.Printf("fromSeq: %s toSeq: %s diff: %d", fromSeq, toSeq, toInt-fromInt)

	return intValue
}

func parseValues(rows pgx.Rows, sender *keypair.Full, receiver string) []int {
	values := make([]int, 0)

	privKey := crypto.StellarKeypairToPrivKey(sender)
	pubKey := crypto.StellarAddressToPubKey(receiver)

	for rows.Next() {
		var (
			memo       string
			accountSeq int64
		)
		err := rows.Scan(&memo, &accountSeq)
		if err != nil {
			log.Fatal(err)
		}
		intValue := parseMemo(memo, accountSeq, privKey, pubKey)
		values = append(values, int(intValue))
	}
	return values
}

func parseMemo(memo string, accountSeq int64, timeKeypair ed25519.PrivateKey, sensorAddress ed25519.PublicKey) int64 {
	// start := time.Now()
	out := make([]byte, base64.StdEncoding.DecodedLen(len(memo)))
	length, err := base64.StdEncoding.Decode(out, []byte(memo))
	if err != nil {
		log.Fatal(err)
	}

	var memoBytes [32]byte
	copy(memoBytes[:], out[:length])

	// encStart := time.Now()
	decrypted, err := crypto.EncryptToMemo(accountSeq, timeKeypair, sensorAddress, memoBytes)
	// encTotal := time.Since(encStart)

	decryptedValue := strings.Trim(string(decrypted[:]), string(rune(0)))
	intValue, _ := strconv.ParseInt(decryptedValue, 10, 32)
	// total := time.Since(start)
	// totalWoEnc := total - encTotal
	// fmt.Printf("%v, %v, %v,\n", total, totalWoEnc, encTotal)

	return intValue
}
