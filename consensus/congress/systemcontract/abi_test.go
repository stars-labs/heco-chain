package systemcontract

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestJsonUnmarshalABI(t *testing.T) {
	for _, abiStr := range []string{ValidatorsInteractiveABI, PunishInteractiveABI, ProposalInteractiveABI, SysGovInteractiveABI, AddrListInteractiveABI} {
		_, err := abi.JSON(strings.NewReader(ValidatorsInteractiveABI))
		require.NoError(t, err, abiStr)
	}
}
