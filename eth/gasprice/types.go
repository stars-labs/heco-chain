package gasprice

import "github.com/ethereum/go-ethereum/core/types"

// TxByPrice sorts the txs descending by price
type TxByPrice types.Transactions

func (s TxByPrice) Len() int { return len(s) }
func (s TxByPrice) Less(i, j int) bool {
	return s[i].GasTipCapCmp(s[j]) > 0 // descending
}
func (s TxByPrice) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
