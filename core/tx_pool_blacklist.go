package core

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

type addrType int

const (
	fromAddr addrType = 0
	toAddr   addrType = 1
)

type blackList struct {
	from map[common.Address]*big.Int
	to   map[common.Address]*big.Int
	lock *sync.RWMutex
}

func newBlackList() *blackList {
	return &blackList{
		from: make(map[common.Address]*big.Int, 0),
		to:   make(map[common.Address]*big.Int, 0),
		lock: &sync.RWMutex{},
	}
}

func (b *blackList) check(addr common.Address, addrType addrType) (exist bool, limit *big.Int, err error) {
	b.lock.RLock()
	defer b.lock.RUnlock()

	var list map[common.Address]*big.Int
	switch addrType {
	case fromAddr:
		list = b.from
	case toAddr:
		list = b.to
	default:
		err = fmt.Errorf("unexpected addr type: %v", addrType)
		return
	}

	limit, exist = list[addr]

	return
}

func (b *blackList) update(newList *BlackList) {
	log.Info("Update blacklist info...")

	b.lock.Lock()
	defer b.lock.Unlock()

	b.from = make(map[common.Address]*big.Int, 0)
	b.to = make(map[common.Address]*big.Int, 0)

	if newList == nil {
		return
	}

	if newList.Froms != nil {
		for addr, limit := range newList.Froms {
			b.from[common.HexToAddress(addr)] = big.NewInt(0).Mul(big.NewInt(int64(params.GWei)), big.NewInt(limit))
		}
	}

	if newList.Tos != nil {
		for addr, limit := range newList.Tos {
			b.to[common.HexToAddress(addr)] = big.NewInt(0).Mul(big.NewInt(int64(params.GWei)), big.NewInt(limit))
		}
	}
}

//名单结构
type BlackList struct {
	Froms map[string]int64 `json:"froms"`
	Tos   map[string]int64 `json:"tos"`
}

//返回结构
type Result struct {
	List *BlackList `json:"list"`
	Code int        `json:"code"` //返回码，0正常，非0 异常
}
