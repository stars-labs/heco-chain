package congress

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

type EventCheckRule struct {
	EventSig common.Hash
	Checks   map[int]common.AddressCheckType
}

type blacklistValidator struct {
	blacks map[common.Address]blacklistDirection
	rules  map[common.Hash]*EventCheckRule
}

func (b *blacklistValidator) IsAddressDenied(address common.Address, cType common.AddressCheckType) (hit bool) {
	d, exist := b.blacks[address]
	if exist {
		switch cType {
		case common.CheckFrom:
			hit = d != DirectionTo // equals to : d == DirectionFrom || d == DirectionBoth
		case common.CheckTo:
			hit = d != DirectionFrom
		case common.CheckBothInAny:
			hit = true
		default:
			log.Warn("blacklist, unsupported AddressCheckType", "type", cType)
			// Unsupported value, not denied by default
			hit = false
		}
	}
	if hit {
		log.Trace("Hit blacklist", "addr", address.String(), "direction", d, "checkType", cType)
	}
	return
}

func (b *blacklistValidator) IsLogDenied(evLog *types.Log) bool {
	if nil == evLog || len(evLog.Topics) <= 1 {
		return false
	}
	if rule, exist := b.rules[evLog.Topics[0]]; exist {
		for idx, checkType := range rule.Checks {
			// do a basic check
			if idx >= len(evLog.Topics) {
				log.Error("check index in rule out to range", "sig", rule.EventSig.String(), "checkIdx", idx, "topicsLen", len(evLog.Topics))
				continue
			}
			addr := common.BytesToAddress(evLog.Topics[idx].Bytes())
			if b.IsAddressDenied(addr, checkType) {
				return true
			}
		}
	}
	return false
}
