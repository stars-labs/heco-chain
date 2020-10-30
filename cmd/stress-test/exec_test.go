package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConcurrentWorks(t *testing.T) {
	// generate new account
	workFn := func(start, end int, data ...interface{}) []interface{} {
		tmpAccounts := make([]interface{}, 0)
		for i := start; i < end; i++ {
			tmpAccounts = append(tmpAccounts, newRandomAccount())
		}

		return tmpAccounts
	}
	accounts := concurrentWork(10, 101, workFn, nil)
	assert.Equal(t, 101, len(accounts))

	accounts = concurrentWork(10, 5, workFn, nil)
	assert.Equal(t, 5, len(accounts))
}
