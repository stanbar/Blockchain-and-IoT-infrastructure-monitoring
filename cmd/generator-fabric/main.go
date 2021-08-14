package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"sync"
	"time"

	// "math/rand"
	// "net/http"
	"os"
	"path/filepath"

	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/gateway"
	"github.com/stellot/stellot-iot/pkg/usecases"
	"github.com/stellot/stellot-iot/pkg/utils"
	"golang.org/x/time/rate"
)

func main() {
	err := os.Setenv("DISCOVERY_AS_LOCALHOST", "true")
	err = os.Setenv("TEMP_ASSET_NAME", "TEMP")
	err = os.Setenv("HUMD_ASSET_NAME", "HUMD")
	if err != nil {
		log.Fatalf("Error setting DISCOVERY_AS_LOCALHOST environemnt variable: %v", err)
	}
	wallet, err := gateway.NewFileSystemWallet("wallet")
	if err != nil {
		log.Fatalf("Failed to create wallet: %v", err)
	}
	if !wallet.Exists("appUser") {
		err = populateWallet(wallet)
		if err != nil {
			log.Fatalf("Failed to populate wallet contents: %v", err)
		}
	}
	ccpPath := filepath.Join(
    "/",
    "home",
    "stanbar",
    "go",
    "src",
    "github.com",
    "stanbar",
    "fabric-samples",
		"test-network",
		"organizations",
		"peerOrganizations",
		"org1.example.com",
		"connection-org1.yaml",
	)
	gw, err := gateway.Connect(
		gateway.WithConfig(config.FromFile(filepath.Clean(ccpPath))),
		gateway.WithIdentity(wallet, "appUser"),
	)
	if err != nil {
		log.Fatalf("Failed to connect to gateway: %v", err)
	}
	defer gw.Close()

	network, err := gw.GetNetwork("mychannel")
	if err != nil {
		log.Fatalf("Failed to get network: %v", err)
	}

	contract := network.GetContract("stelliot")

	log.Println("--> Evaluate Transaction: GetAggregation, function returns aggregation")
	result, err := contract.EvaluateTransaction("GetAggregation", "asdf", "2021-06-05T18")
	if err != nil {
		log.Fatalf("Failed to evaluate transaction: %v", err)
	}
	log.Println(string(result))

	sensorDevices := createSensorDevices()

	err = startGenerator(contract, sensorDevices)

	if err != nil {
		log.Fatalf("Failed to start generator: %v", err)
	}
}

func populateWallet(wallet *gateway.Wallet) error {
	log.Println("============ Populating wallet ============")
	credPath := filepath.Join(
    "/",
    "home",
    "stanbar",
    "go",
    "src",
    "github.com",
    "stanbar",
    "fabric-samples",
		"test-network",
		"organizations",
		"peerOrganizations",
		"org1.example.com",
		"users",
		"Admin@org1.example.com",
		"msp",
	)

	certPath := filepath.Join(credPath, "signcerts", "Admin@org1.example.com-cert.pem")
	// read the certificate pem
	cert, err := ioutil.ReadFile(filepath.Clean(certPath))
	if err != nil {
		return err
	}

	keyDir := filepath.Join(credPath, "keystore")
	// there's a single file in this dir containing the private key
	files, err := ioutil.ReadDir(keyDir)
	if err != nil {
		return err
	}
	if len(files) != 1 {
		return fmt.Errorf("keystore folder should have contain one file")
	}
	keyPath := filepath.Join(keyDir, files[0].Name())
	key, err := ioutil.ReadFile(filepath.Clean(keyPath))
	if err != nil {
		return err
	}

	identity := gateway.NewX509Identity("Org1MSP", string(cert), string(key))

	return wallet.Put("appUser", identity)
}

type sensorDevice struct {
	DeviceId    int
	LogValue    [32]byte
	PhysicsType usecases.PhysicsType
	Server      string
	RateLimiter *rate.Limiter
}

var (
	LogsNumber, _          = strconv.Atoi(utils.MustGetenv("LOGS_NUMBER"))
)

func startGenerator(contract *gateway.Contract, iotDevices []sensorDevice) error {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, iotDevice := range iotDevices {
		wg.Add(1)
		go func(params sensorDevice, wg *sync.WaitGroup) {
			defer wg.Done()
			time.Sleep(time.Duration(1000.0*params.DeviceId/len(iotDevices)) * time.Millisecond)
			for i := 0; i < LogsNumber; i++ {
				select {
				case <-ctx.Done():
					log.Printf("Sensor %d received cancelation signal, exiting \n", params.DeviceId)
					return
				default: // Default is must to avoid blocking
				}
				err := params.RateLimiter.Wait(ctx)
				if err != nil {
					cancel()
					log.Println("Error returned by limiter", err)
					return
				}
				err = sendLogTx(contract, params, i)
				if err != nil {
					cancel()
					return
				}
			}
		}(iotDevice, &wg)
	}
	log.Println("Waiting for workers to finish")
	wg.Wait()
	log.Println("Completed")
	return ctx.Err()
}

func sendLogTx(contract *gateway.Contract, params sensorDevice, i int) error {
	deviceId := strconv.Itoa(params.DeviceId)
	logValue := strconv.Itoa(params.PhysicsType.RandomValueInt())
	physicsType := params.PhysicsType.Asset().GetCode()
	creationTime := time.Now().Format(time.RFC3339)
	log.Printf("--> Submit Transaction: SetSensorState, %v %v %v %s", deviceId, logValue, physicsType, creationTime)

	result, err := contract.SubmitTransaction("SetSensorState", deviceId, logValue, physicsType, creationTime)

	if err != nil {
		log.Fatalf("Failed to Submit transaction: %v", err)
		return err
	}
	log.Println(string(result))
	return nil
}

func createSensorDevices() []sensorDevice {
	iotDevices := make([]sensorDevice, 50)
	for i := 0; i < 50; i++ {
		physicType := usecases.TEMP
		if i%2 == 1 {
			physicType = usecases.HUMD
		}

		iotDevices[i] = sensorDevice{
			DeviceId:    i,
			PhysicsType: physicType,
			RateLimiter: rate.NewLimiter(rate.Every(time.Duration(1000.0)*time.Millisecond), 1),
		}
	}
	return iotDevices
}
