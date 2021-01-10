package utils

import (
	"log"
	"os"

	"github.com/stellar/go/keypair"
)

// MustGetenv returns os.Lookup or panic
func MustGetenv(key string) string {
	var value, present = os.LookupEnv(key)
	if !present {
		log.Panicf("%s must be specified\n", key)
	}
	return value
}

// Return chunked keypair slice
func ChunkKeypairs(slice []*keypair.Full, chunkSize int) [][]*keypair.Full {
	var chunks [][]*keypair.Full
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
