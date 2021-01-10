package usecases

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func TestDeriveDHKeyRandom(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < 24; i++ {
		fmt.Printf("%d %s\n", i, RandomTemperature(i))
	}
}
