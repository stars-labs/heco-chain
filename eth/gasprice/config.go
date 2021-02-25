package gasprice

import "math/big"

type Config struct {
	Blocks     int
	Percentile int
	Default    *big.Int `toml:",omitempty"`
	MaxPrice   *big.Int `toml:",omitempty"`

	PredictIntervalSecs int
	MinTxCntPerBlock    int // minimum tx cnt per block for caculations.
	MaxMedianIndex      int // max index in all pending transactions for median price
	MaxLowIndex         int // max index in all pending transactions for low price

	FastPercentile   int //fast percentile for the case there are no many pending transactions
	MeidanPercentile int
}
