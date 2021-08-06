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
	conn, err := pgxpool.Connect(context.Background(), helpers.DatabaseUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}

	defer conn.Close()

	// 1. GET ALL TRANSACTIONS WHERE
	values := get1Agg(conn, helpers.DevicesKeypairs[0].Address(), functions.AVG, aggregator.ONE_DAY, 1)
	log.Println(len(values))
	values = get1Agg(conn, helpers.DevicesKeypairs[0].Address(), functions.AVG, aggregator.ONE_DAY, 2)
	log.Println(len(values))
	values = get1Agg(conn, helpers.DevicesKeypairs[0].Address(), functions.AVG, aggregator.ONE_DAY, 3)
	log.Println(len(values))

	end3 := time.Unix(1617898929, 0)
	from3 := end3.AddDate(0, 0, -1)

	end2 := from3.AddDate(0, 0, -1)
	from2 := end2.AddDate(0, 0, -1)

	end1 := from2.AddDate(0, 0, -1)
	from1 := end1.AddDate(0, 0, -1)

	froms := []time.Time{from1, from2, from3}
	tos := []time.Time{end1, end2, end3}

	values = get1Tx(conn, helpers.DevicesKeypairs[0].Address(), functions.AVG, froms, tos)
	log.Println(len(values))

	// 2. GET ALL TRANSACTIONS WHERE $value_{min}$ $<$ $sensor_n.value$ $<$ $value_{max}$
	res := get2(conn, helpers.DevicesKeypairs[0], "<", 700)
	log.Println("max < 700", res)

	// 3. GET AVG($sensor_N$, $unit$) WHERE $created\_at$ $>$ time AND $created\_at$ $<$ $time$
	endOfCampaing := time.Unix(1617898929, 0)
	// from := endOfCampaing.Add(-60 * time.Minute)
	from := endOfCampaing.AddDate(0, 0, -1)
	value := get3Tx(conn, helpers.DevicesKeypairs[0].Address(), from, endOfCampaing)
	log.Println("[TX] one day", value)

	value = get3Agg(conn, helpers.DevicesKeypairs[0].Address(), functions.AVG, aggregator.ONE_DAY, 1, false)
	log.Println("[AGG] one day", value)

	value = get3Agg(conn, helpers.DevicesKeypairs[0].Address(), functions.AVG, aggregator.ONE_DAY, 2, false)
	log.Println("[AGG] two days", value)

	from = endOfCampaing.AddDate(0, 0, -2)
	value = get3Tx(conn, helpers.DevicesKeypairs[0].Address(), from, endOfCampaing)
	log.Println("[TX] two days", value)

	value = get3Agg(conn, helpers.DevicesKeypairs[0].Address(), functions.AVG, aggregator.ONE_DAY, 3, false)
	log.Println("[AGG] three days", value)

	from = endOfCampaing.AddDate(0, 0, -3)
	value = get3Tx(conn, helpers.DevicesKeypairs[0].Address(), from, endOfCampaing)
	log.Println("[TX] three days", value)

	// 4. GET COUNT(*) WHERE SENSOR = $sensor_N$ AND UNIT=$unit$ AND $created\_at$ $>$ $time$
	get4Agg(conn, helpers.DevicesKeypairs[0].Address(), usecases.TEMP, functions.AVG, 1, aggregator.SIX_HOURS)
	get4Tx(conn, helpers.DevicesKeypairs[0].Address(), 1, aggregator.SIX_HOURS)

}

func get4Agg(conn *pgxpool.Pool, sensorAddress string, usecase usecases.PhysicsType, function functions.FunctionType, lastTxs int, timeInterval aggregator.TimeInterval) {
	defer utils.Duration(utils.Track("countTxs"))
	start := time.Now()

	from := aggregator.ByTimeInterval(timeInterval).Keypair.Address()
	to := sensorAddress
	assetCode := function.Asset().GetCode()
	offset := lastTxs

	log.Println("from: ", from, " to: ", to, " assetCode: ", assetCode, " offset: ", offset)

	row := conn.QueryRow(context.Background(), `
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
		log.Fatal(err)
	}
	log.Println("txId: ", txId)

	rows, err := conn.Query(context.Background(), `
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
		log.Fatal(err)
	}

	rows.Next()
	err = rows.Scan(&toSeq)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("fromSeq: %s toSeq: %s", fromSeq, toSeq)
	fromInt, err := strconv.ParseUint(fromSeq, 10, 64)
	toInt, err := strconv.ParseUint(toSeq, 10, 64)
	log.Println("Pre interpreted count: ", toInt-fromInt)
}

func get4Tx(conn *pgxpool.Pool, sensorAddress string, lastTxs int, timeInterval aggregator.TimeInterval) {
	defer utils.Duration(utils.Track("countTxsTimeBounds"))
	start := time.Now()
	to := sensorAddress

	sixHoursAgoUnix := time.Date(2021, 3, 24, 16, 0, 0, 0, time.UTC).Unix() - 100000
	row := conn.QueryRow(context.Background(), `
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
	}

	log.Println("count: ", count)
}

func get2(conn *pgxpool.Pool, sensor *keypair.Full, operation string, predicate int) []int {
	defer utils.Duration(utils.Track("getValuesPredicateTx"))
	from := sensor.Address()
	to := helpers.BatchKeypair.Address()

	log.Println("from: ", from, " to: ", to)

	start := time.Now()
	rows, err := conn.Query(context.Background(), `
  SELECT memo, account_sequence FROM history_operations ops
  JOIN history_transactions txs on ops.transaction_id = txs.id
  WHERE type = 1
    AND details->>'from' = $1
    AND details->>'to' = $2
  `, from, to)
	log.Println("execute sql", time.Since(start))

	if err != nil {
		log.Fatalln(err)
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

func get1Agg(conn *pgxpool.Pool, sensorAddress string, function functions.FunctionType, timeInterval aggregator.TimeInterval, lastTxs int) []int {
	defer utils.Duration(utils.Track("get 1 agg"))
	start := time.Now()

	from := aggregator.ByTimeInterval(timeInterval).Keypair.Address()
	to := sensorAddress
	assetCode := function.Asset().GetCode()
	offset := lastTxs

	log.Println("from: ", from, " to: ", to, " assetCode: ", assetCode, " offset: ", offset)

	row := conn.QueryRow(context.Background(), `
  SELECT transaction_id FROM history_operations ops
  JOIN history_transactions txs on ops.transaction_id = txs.id
  WHERE source_account = $1
    AND type = 1
    AND details->>'from' = $2
    AND details->>'to' = $3
    AND details->>'asset_code' = $4
  LIMIT 1
  OFFSET $5;
  `, from, from, to, assetCode, offset)
	log.Println("executed sql", time.Since(start))

	var txId int64
	err := row.Scan(&txId)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("txId: ", txId)

	rows, err := conn.Query(context.Background(), `
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
		log.Fatal(err)
	}

	rows.Next()
	err = rows.Scan(&toSeq)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("fromSeq: %s toSeq: %s", fromSeq, toSeq)
	fromInt, err := strconv.ParseUint(fromSeq, 10, 64)
	toInt, err := strconv.ParseUint(toSeq, 10, 64)
	log.Println("Pre interpreted count: ", toInt-fromInt)

	rows, err = conn.Query(context.Background(), `
  SELECT memo, account_sequence FROM history_transactions txs
  WHERE txs.account = $1
    AND account_sequence >= $2 
    AND account_sequence < $3;
  `, sensorAddress, fromInt, toInt)
	log.Println(sensorAddress, fromInt, toInt)
	values := parseValues(rows, helpers.BatchKeypair, sensorAddress)
	if err != nil {
		log.Fatalln(err)
	}
	return values
}

func get1Tx(conn *pgxpool.Pool, sensorAddress string, function functions.FunctionType, from []time.Time, to []time.Time) []int {
	defer utils.Duration(utils.Track("get 1 tx"))
	rows, err := conn.Query(context.Background(), `
	SELECT memo, account_sequence FROM history_transactions txs
	WHERE txs.account = $1
    AND (
         (lower(txs.time_bounds) > $2 AND lower(txs.time_bounds) <= $3)
      OR (lower(txs.time_bounds) > $4 AND lower(txs.time_bounds) <= $5)
      OR (lower(txs.time_bounds) > $6 AND lower(txs.time_bounds) <= $7)
    )
    ;
	`, sensorAddress, from[0].Unix(), to[0].Unix(), from[1].Unix(), to[1].Unix(), from[2].Unix(), to[2].Unix())

	if err != nil {
		panic(err)
	}

	values := parseValues(rows, helpers.BatchKeypair, sensorAddress)
	return values
}

func get3Tx(conn *pgxpool.Pool, sensorAddress string, from time.Time, to time.Time) int {
	defer utils.Duration(utils.Track("get3Tx"))
	start := time.Now()

	// sixHoursAgoUnix := time.Date(2021, 3, 24, 16, 0, 0, 0, time.UTC).Unix() - 100000
	rows, err := conn.Query(context.Background(), `
	SELECT memo, account_sequence FROM history_transactions txs
	WHERE txs.account = $1
    AND lower(txs.time_bounds) > $2
    AND lower(txs.time_bounds) <= $3;
	`, sensorAddress, from.Unix(), to.Unix())

	if err != nil {
		log.Fatal(err)
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

func get3Agg(conn *pgxpool.Pool, sensorAddress string, function functions.FunctionType, timeInterval aggregator.TimeInterval, lastBlocks int, debugging bool) int {
	defer utils.Duration(utils.Track("get3Agg"))
	start := time.Now()
	keypair := aggregator.ByTimeInterval(timeInterval).Keypair
	rows, err := conn.Query(context.Background(), `
  SELECT memo, account_sequence, transaction_id FROM history_operations ops
  JOIN history_transactions txs on ops.transaction_id = txs.id
  WHERE type = 1
    AND details->>'from' = $1
    AND details->>'to' = $2
    AND details->>'asset_code' = $3
  ORDER BY account_sequence DESC
  LIMIT $4
  `, keypair.Address(), sensorAddress, function.Asset().GetCode(), lastBlocks)
	elapsed := time.Since(start)
	log.Println("executed sql", elapsed)
	if err != nil {
		log.Fatal(err)
	}

	privKey := crypto.StellarKeypairToPrivKey(keypair)
	pubKey := crypto.StellarAddressToPubKey(sensorAddress)

	var (
		memo       string
		accountSeq int64
		txId       int64
		sum        int64
		blocks     int64
		startSeq   *int64
		endSeq     int64
	)
	for rows.Next() {
		err := rows.Scan(&memo, &accountSeq, &txId)
		if err != nil {
			log.Println("Error")
			log.Println(err)
			log.Fatal(err)
		}
		intValue := parseMemo(memo, accountSeq, privKey, pubKey)
		sum += intValue
		blocks += 1

		if !debugging {
			continue
		}
		var (
			fromSeq string
			toSeq   string
		)

		err = conn.QueryRow(context.Background(), `
  SELECT details->>'bump_to' FROM history_operations ops
  WHERE transaction_id = $1
    AND details->>'bump_to' IS NOT NULL
  ORDER BY application_order
  LIMIT 1;
  `, txId).Scan(&fromSeq)

		if err != nil {
			log.Fatal(err)
		}

		err = conn.QueryRow(context.Background(), `
  SELECT details->>'bump_to' FROM history_operations ops
  WHERE transaction_id = $1
    AND details->>'bump_to' IS NOT NULL
  ORDER BY application_order
  LIMIT 1;
  `, txId).Scan(&toSeq)
		if err != nil {
			log.Fatal(err)
		}
		if startSeq == nil {
			parsed, _ := strconv.ParseInt(fromSeq, 10, 64)
			startSeq = &parsed
		}
		endSeq, _ = strconv.ParseInt(toSeq, 10, 64)
		log.Printf("partial: startSeq: %d endSeq: %d diff: %d", *startSeq, endSeq, *startSeq-endSeq)
	}
	if debugging {
		log.Printf("total: startSeq: %d endSeq: %d diff: %d", *startSeq, endSeq, *startSeq-endSeq)
	}

	return int(sum / blocks)
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
