package temperature

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func TestDeriveDHKeyRandom(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	for hour := 0; hour < 24; hour++ {
		for minute := 0; minute < 5; minute++ {
			now = func() time.Time {
				return time.Date(2020, 1, 1, hour, minute*10, 0, 0, time.Local)
			}
			fmt.Printf("%02d %02d %s\n", hour, minute*10, RandomTemperature())
		}
	}
}
