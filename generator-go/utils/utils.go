package utils

import (
	"log"
	"os"
)

// MustGetenv returns os.Lookup or panic
func MustGetenv(key string) string {
	var value, present = os.LookupEnv(key)
	if !present {
		log.Panicf("%s must be specified\n", key)
	}
	return value
}
