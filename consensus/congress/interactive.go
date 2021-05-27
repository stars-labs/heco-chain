package congress

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
)

type chainContext struct {
	chainReader consensus.ChainHeaderReader
	engine      consensus.Engine
}

func newChainContext(chainReader consensus.ChainHeaderReader, engine consensus.Engine) *chainContext {
	return &chainContext{
		chainReader: chainReader,
		engine:      engine,
	}
}

// Engine retrieves the chain's consensus engine.
func (cc *chainContext) Engine() consensus.Engine {
	return cc.engine
}

// GetHeader returns the hash corresponding to their hash.
func (cc *chainContext) GetHeader(hash common.Hash, number uint64) *types.Header {
	return cc.chainReader.GetHeader(hash, number)
}

// minimalChainContext provides a `core.ChainContext` implementation without really functioned `GetHeader` method,
// it's used to execute those contracts which do no includes `BLOCKHASH` opcode.
// The purpose is to reduce dependencies between different packages.
type minimalChainContext struct {
	engine consensus.Engine
}

func newMinimalChainContext(engine consensus.Engine) *minimalChainContext {
	return &minimalChainContext{
		engine: engine,
	}
}

// Engine retrieves the chain's consensus engine.
func (cc *minimalChainContext) Engine() consensus.Engine {
	return cc.engine
}

func (cc *minimalChainContext) GetHeader(hash common.Hash, number uint64) *types.Header {
	return nil
}
