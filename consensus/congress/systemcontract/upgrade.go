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
	sysContracts []IUpgradeAction
)

func init() {
	sysContracts = []IUpgradeAction{
		&hardForkSysGov{},
		&hardForkAddressList{},
		&hardForkValidatorsV1{},
		&hardForkPunishV1{},
	}
}

func ApplySystemContractUpgrade(state *state.StateDB, header *types.Header, chainContext core.ChainContext, config *params.ChainConfig) (err error) {
	if config == nil || header == nil || state == nil {
		return
	}
	height := header.Number

	for _, contract := range sysContracts {
		log.Info("system contract upgrade", "name", contract.GetName(), "height", height, "chainId", config.ChainID.String())

		err = contract.Update(config, height, state)
		if err != nil {
			log.Error("Upgrade system contract update error", "name", contract.GetName(), "err", err)
			return
		}

		log.Info("system contract upgrade execution", "name", contract.GetName(), "height", header.Number, "chainId", config.ChainID.String())

		err = contract.Execute(state, header, chainContext, config)
		if err != nil {
			log.Error("Upgrade system contract execute error", "name", contract.GetName(), "err", err)
			return
		}
	}
	// Update the state with pending changes
	state.Finalise(true)

	return
}
