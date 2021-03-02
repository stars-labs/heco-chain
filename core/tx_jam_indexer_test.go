package core

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestJamIndexer(t *testing.T) {
	idxer := NewTxJamIndexer(TxJamConfig{})
	for i := 0; i < 5000; i++ {
		tx := types.NewTransaction(uint64(i), common.Address{byte(i)}, big.NewInt(int64(i)), 1000000, big.NewInt(1e9), nil)
		idxer.PendingIn(types.Transactions{tx})
		if i%100 == 0 {
			idxer.UnderPricedInc()
		}
	}
	for i := 0; i < 9; i++ {
		t.Log(idxer.JamIndex())
		time.Sleep(3 * time.Second)
	}
	idxer.Stop()
}
