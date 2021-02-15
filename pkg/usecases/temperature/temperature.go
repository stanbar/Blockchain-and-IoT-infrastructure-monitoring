package temperature

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"time"
)

const (
	baseTemp   = 10.0
	baseTempAt = 10.0
	amp        = 7.5
)

func SinDay(hourDotMinutes float64) float64 {
	return baseTemp + amp*math.Sin(math.Pi/12.0*(hourDotMinutes-baseTempAt))
}

func MutByWeek(temp float64, day int) float64 {
	return temp + math.Sin(float64(day)*math.Pi/3.5)/2
}

func MutByDeviation(temp float64) float64 {
	return temp + temp*(rand.Float64()-0.5)/70
}

var now = time.Now

func RandomTemperature() [32]byte {
	now := now()
	fmt.Println(now)

	hourDotMinutes := float64(now.Hour()) + (0.0169491 * float64(now.Minute()))

	temp := SinDay(hourDotMinutes)
	temp = MutByWeek(temp, int(now.Weekday()))
	temp = MutByDeviation(temp)
	fmt.Println("temp", temp)

	multipled := temp * 10
	result := int(math.Round(multipled))
	var output [32]byte
	copy(output[:], strconv.Itoa(result))
	return output
}
