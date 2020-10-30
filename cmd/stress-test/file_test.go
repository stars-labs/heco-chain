package main

import (
	"crypto/ecdsa"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestWriteAndLoadAccounts(t *testing.T) {
	account, _ := crypto.GenerateKey()

	path := "/tmp/tmp"

	err := writeAccounts(path, []*ecdsa.PrivateKey{account})
	require.Nil(t, err)

	actual, err := loadAccounts(path)
	require.Nil(t, err)
	require.Equal(t, 1, len(actual))

	require.True(t, account.D.Cmp(actual[0].D) == 0)
}
