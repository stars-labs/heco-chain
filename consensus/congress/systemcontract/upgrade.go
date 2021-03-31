package systemcontract

import (
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"math/big"
)

type IUpgradeAction interface {
	GetName() string
	Update(config *params.ChainConfig, height *big.Int, state *state.StateDB) error
	Execute(state *state.StateDB, header *types.Header, chainContext core.ChainContext, config *params.ChainConfig) error
}

var (
	sysGov IUpgradeAction
)

func init() {
	sysGov = &hardForkSysGov{}
}

func ApplySystemContractUpgrade(config *params.ChainConfig, height *big.Int, state *state.StateDB) {
	if config == nil || height == nil || state == nil {
		return
	}

	if config.IsSysGov(height) {
		log.Info("system contract upgrade", "name", sysGov.GetName(), "height", height, "chainId", config.ChainID.String())

		err := sysGov.Update(config, height, state)
		if err != nil {
			log.Crit("Upgrade system contract error", "name", sysGov.GetName(), "err", err)
		}
	}

	return
}


func ApplySystemContractExecution(state *state.StateDB, header *types.Header, chainContext core.ChainContext, config *params.ChainConfig) {
	if config == nil || header == nil || state == nil {
		return
	}

	if config.IsSysGov(header.Number) {
		log.Info("system contract upgrade execution", "name", sysGov.GetName(), "height", header.Number, "chainId", config.ChainID.String())

		err := sysGov.Execute(state, header, chainContext, config)
		//todo
		if err != nil {
			log.Crit("Upgrade system contract error", "name", sysGov.GetName(), "err", err)
		}
	}

	return
}
