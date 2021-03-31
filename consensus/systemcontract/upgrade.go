package systemcontract

import (
	"github.com/ethereum/go-ethereum/consensus/systemcontract/hardfork"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"math/big"
)

type IUpgradeAction interface {
	GetName() string
	Execute(config *params.ChainConfig, height *big.Int, state *state.StateDB) error
}

var (
	sysGov IUpgradeAction
)

func init() {
	sysGov = &hardfork.SysGov{}
}

func ApplySystemContractUpgrade(config *params.ChainConfig, height *big.Int, state *state.StateDB) {
	if config == nil || height == nil || state == nil {
		return
	}

	if config.IsSysGov(height) {
		log.Info("system contract upgrade", "name", sysGov.GetName())

		err := sysGov.Execute(config, height, state)
		if err != nil {
			log.Crit("Upgrade system contract error", "name", sysGov.GetName(), "err", err)
		}
	}

	return
}
