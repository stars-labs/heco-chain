package gasprice

type PredConfig struct {
	PredictIntervalSecs int
	MinTxCntPerBlock    int // minimum tx cnt per block for caculations.
	FastFactor          int // how many times of avgTxCnt for the fast index
	MedianFactor        int // how many times of avgTxCnt for the median index
	LowFactor           int // how many times of avgTxCnt for the low index
	MinMedianIndex      int // min index in all pending transactions for median price
	MinLowIndex         int // min index in all pending transactions for low price

	FastPercentile   int //fast percentile for the case there are no many pending transactions
	MeidanPercentile int

	MaxValidPendingSecs int
}
