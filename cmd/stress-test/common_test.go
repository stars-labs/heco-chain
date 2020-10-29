package main

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

func TestPackData(t *testing.T) {
	to := common.HexToAddress("0xe244fc5ba65bf70a84b9966579e105c5c57429c5")
	amount := big.NewInt(32)
	amount.Mul(amount, big.NewInt(params.Ether))

	expect, _ := hex.DecodeString("a9059cbb000000000000000000000000e244fc5ba65bf70a84b9966579e105c5c57429c5000000000000000000000000000000000000000000000001bc16d674ec800000")
	actual := packData(to, amount)
	if !bytes.Equal(actual, expect) {
		t.Fatalf("pack data not equal, expect: %v, actual: %v", expect, actual)
	}
}
