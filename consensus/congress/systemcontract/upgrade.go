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

func ApplySystemContractUpgrade(config *params.ChainConfig, height *big.Int, state *state.StateDB) (err error) {
	if config == nil || height == nil || state == nil {
		return
	}

	if config.SysGovBlock.Cmp(height) == 0 {
		log.Info("system contract upgrade", "name", sysGov.GetName(), "height", height, "chainId", config.ChainID.String())

		err = sysGov.Update(config, height, state)
		if err != nil {
			log.Error("Upgrade system contract update error", "name", sysGov.GetName(), "err", err)
			return
		}
	}

	return
}

func ApplySystemContractExecution(state *state.StateDB, header *types.Header, chainContext core.ChainContext, config *params.ChainConfig) (err error) {
	if config == nil || header == nil || state == nil {
		return
	}

	if config.SysGovBlock.Cmp(header.Number) == 0 {
		log.Info("system contract upgrade execution", "name", sysGov.GetName(), "height", header.Number, "chainId", config.ChainID.String())

		err = sysGov.Execute(state, header, chainContext, config)
		if err != nil {
			log.Error("Upgrade system contract execute error", "name", sysGov.GetName(), "err", err)
			return
		}
	}

	return
}
