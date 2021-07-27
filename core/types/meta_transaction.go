package types

import (
	"encoding/hex"
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"math/big"
	"strconv"
)

var (
	ErrInvalidMetaSig     = errors.New("meta transaciont verify: invalid transaction v, r, s values")
	ErrInvalidMetaDataLen = errors.New("invalid metadata length")

	MetaPrefix         = "234d6574615472616e73616374696f6e23"
	BIG10000           = new(big.Int).SetUint64(10000)
	MetaPrefixBytesLen = 17
)

type MetaData struct {
	//fee cover percentage, 0-10000, 0: means no cover. 1: means cover 0.01%, 10000 means full cover
	BlockNumLimit uint64 `json:"blockNumLimit" gencodec:"required"`
	FeePercent    uint64 `json:"feepercent" gencodec:"required"`
	// Signature values
	V       *big.Int `json:"v" gencodec:"required"`
	R       *big.Int `json:"r" gencodec:"required"`
	S       *big.Int `json:"s" gencodec:"required"`
	Payload []byte   `json:"input"    gencodec:"required"`
}

func IsMetaTransaction(data []byte) bool {
	if len(data) >= MetaPrefixBytesLen {
		prefix := hex.EncodeToString(data[:MetaPrefixBytesLen])
		return prefix == MetaPrefix
	}
	return false
}

func DecodeMetaData(encodedData []byte, blockNumber *big.Int) (*MetaData, error) {
	metaData := new(MetaData)
	if len(encodedData) <= MetaPrefixBytesLen {
		return metaData, ErrInvalidMetaDataLen
	}
	encodedData = encodedData[MetaPrefixBytesLen:]
	if err := rlp.DecodeBytes(encodedData, metaData); err != nil {
		return metaData, err
	}
	if metaData.FeePercent > BIG10000.Uint64() {
		return metaData, errors.New("invalid meta transaction FeePercent need 0-10000. Found:" + strconv.FormatUint(metaData.FeePercent, 10))
	}
	if metaData.BlockNumLimit < blockNumber.Uint64() {
		return metaData, errors.New("expired meta transaction. current:" + strconv.FormatUint(blockNumber.Uint64(), 10) + ", need execute before " + strconv.FormatUint(metaData.BlockNumLimit, 10))
	}
	return metaData, nil
}

func (metadata *MetaData) ParseMetaData(nonce uint64, gasPrice *big.Int, gas uint64, to *common.Address, value *big.Int, payload []byte, from common.Address, chainID *big.Int) (common.Address, error) {
	var data interface{} = []interface{}{
		nonce,
		gasPrice,
		gas,
		to,
		value,
		payload,
		from,
		metadata.FeePercent,
		metadata.BlockNumLimit,
		chainID,
	}
	raw, _ := rlp.EncodeToBytes(data)
	log.Debug("meta rlpencode" + hexutil.Encode(raw[:]))
	hash := rlpHash(data)
	log.Debug("meta rlpHash", hexutil.Encode(hash[:]))

	var big8 = big.NewInt(8)
	chainMul := new(big.Int).Mul(chainID, big.NewInt(2))
	V := new(big.Int).Sub(metadata.V, chainMul)
	V.Sub(V, big8)
	addr, err := RecoverPlain(hash, metadata.R, metadata.S, V, true)
	if err != nil {
		return common.HexToAddress(""), ErrInvalidMetaSig
	}
	return addr, nil
}
