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
var HumdAssetName = utils.MustGetenv("HUMD_ASSET_NAME")

var AssetKeypair = keypair.MustParseFull(utils.MustGetenv("ASSET_ISSUER_SECRET"))
var AssetIssuer = utils.MustGetenv("ASSET_ISSUER_PUBLIC")

func (pt PhysicsType) RandomValue(offset int) [32]byte {
	if pt == TEMP {
		return RandomTemperature(offset)
	}
	return RandomHumidity(offset)
}

func (pt PhysicsType) Asset() txnbuild.Asset {
	if pt == TEMP {
		return txnbuild.CreditAsset{Code: TempAssetName, Issuer: AssetIssuer}
	}
	return txnbuild.CreditAsset{Code: HumdAssetName, Issuer: AssetIssuer}
}
