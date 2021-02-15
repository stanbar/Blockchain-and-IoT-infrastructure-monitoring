package humidity

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func TestDeriveDHKeyRandom(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	for hour := 0; hour < 24; hour++ {
		for minute := 0; minute < 59; minute++ {
			now = func() time.Time {
				return time.Date(2020, 1, 1, hour, minute, 0, 0, time.Local)
			}

			fmt.Printf("%d %s\n", hour, RandomHumidity())
		}
	}
}
