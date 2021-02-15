package humidity

import (
	"math"
	"math/rand"
	"strconv"
	"time"
)

const (
	baseHum   = 60.0
	baseHumAt = 10.0
	amp       = 20
)

func SinValue(hourDotMinutes float64) float64 {
	return baseHum + amp*math.Cos(math.Pi/12.0*(hourDotMinutes-baseHumAt)+math.Pi)
}

var now = time.Now

func RandomHumidity() [32]byte {
	now := now()
	hourDotMinutes := float64(now.Hour()) + (0.0169491 * float64(now.Minute()))

	temp := SinValue(hourDotMinutes)
	temp = temp + temp*rand.Float64()

	multipled := temp * 10
	result := int(math.Round(multipled))
	var output [32]byte
	copy(output[:], strconv.Itoa(result))
	return output
}
