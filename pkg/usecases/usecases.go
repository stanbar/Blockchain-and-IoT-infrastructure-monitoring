package usecases

import (
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
	"github.com/stellot/stellot-iot/pkg/utils"
)

type PhysicsType uint

const (
	TEMP PhysicsType = iota
	HUMD
)

var TempAssetName = utils.MustGetenv("TEMP_ASSET_NAME")
var TempAssetIssuer = utils.MustGetenv("TEMP_ASSET_ISSUER_PUBLIC")
var TempAssetKeypair = keypair.MustParseFull(utils.MustGetenv("TEMP_ASSET_ISSUER_SECRET"))

var HumdAssetName = utils.MustGetenv("HUMD_ASSET_NAME")
var HumdAssetIssuer = utils.MustGetenv("HUMD_ASSET_ISSUER_PUBLIC")
var HumdAssetKeypair = keypair.MustParseFull(utils.MustGetenv("HUMD_ASSET_ISSUER_SECRET"))

func (pt PhysicsType) RandomValue(offset int) [32]byte {
	if pt == TEMP {
		return RandomTemperature(offset)
	}
	return RandomHumidity(offset)
}

func (pt PhysicsType) Asset() txnbuild.Asset {
	if pt == TEMP {
		return txnbuild.CreditAsset{Code: TempAssetName, Issuer: TempAssetIssuer}
	}
	return txnbuild.CreditAsset{Code: HumdAssetName, Issuer: HumdAssetIssuer}
}
