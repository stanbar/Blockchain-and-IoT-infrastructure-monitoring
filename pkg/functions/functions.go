package functions

import (
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
	"github.com/stellot/stellot-iot/pkg/utils"
)

type FunctionType uint

const (
	AVG FunctionType = iota
	MIN
	MAX
	FROM
	TO
)

var (
	AvgAssetName  = utils.MustGetenv("AVG_ASSET_NAME")
	MinAssetName  = utils.MustGetenv("MIN_ASSET_NAME")
	MaxAssetName  = utils.MustGetenv("MAX_ASSET_NAME")
	FromAssetName = utils.MustGetenv("FROM_ASSET_NAME")
	ToAssetName   = utils.MustGetenv("TO_ASSET_NAME")

	AssetKeypair = keypair.MustParseFull(utils.MustGetenv("FUNCTIONS_ASSET_ISSUER_SECRET"))
	AssetIssuer  = AssetKeypair.Address()
	Assets       = []txnbuild.Asset{AVG.Asset(), MIN.Asset(), MAX.Asset(), FROM.Asset(), TO.Asset()}
)

func (ft FunctionType) Asset() txnbuild.Asset {
	if ft == AVG {
		return txnbuild.CreditAsset{Code: AvgAssetName, Issuer: AssetIssuer}
	} else if ft == MIN {
		return txnbuild.CreditAsset{Code: MinAssetName, Issuer: AssetIssuer}
	} else if ft == FROM {
		return txnbuild.CreditAsset{Code: FromAssetName, Issuer: AssetIssuer}
	} else if ft == TO {
		return txnbuild.CreditAsset{Code: ToAssetName, Issuer: AssetIssuer}
	}
	return txnbuild.CreditAsset{Code: MaxAssetName, Issuer: AssetIssuer}
}
