package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v4/pgxpool"
)

func main() {
	dbpool, err := pgxpool.Connect(context.Background(), "postgres://stellar:jBH7qeurzt1wOCQ2@localhost/core")

	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer dbpool.Close()

	rows, err := dbpool.Query(context.Background(), "SELECT accountid, balance FROM accounts;")
	for rows.Next() {
		var accountid string
		var balance int
		err := rows.Scan(&accountid, &balance)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s, %d\n", accountid, balance)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "QueryRow failed: %v\n", err)
		os.Exit(1)
	}
}
