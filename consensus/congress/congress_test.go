package congress

import (
	"github.com/ethereum/go-ethereum/common"
	"testing"
)

func TestCalcSlotOfDevMappingKey(t *testing.T) {
	addr := common.HexToAddress("0x5b38da6a701c568545dcfcb03fcb875f56beddc4")
	slot := calcSlotOfDevMappingKey(addr)
	t.Log(slot.String())
	// want: 0xb314f101a00aa0d8cc6704cc6dd1e9dd7551ec98c9df52079c192c560ba66c4a

}
