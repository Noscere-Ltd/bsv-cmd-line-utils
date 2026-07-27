package utxo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnspentProviders(t *testing.T) {
	t.Run("mainnet tries BananaBlocks before WhatsOnChain", func(t *testing.T) {
		providers := unspentProviders("1BitcoinEaterAddressDontSendf59kuE", false)

		require.Len(t, providers, 2)
		assert.Equal(t,
			"https://bananablocks.com/api/v1/bsv/main/address/1BitcoinEaterAddressDontSendf59kuE/unspent/all?limit=1000",
			providers[0].url)
		assert.Equal(t, bananaMaxLimit, providers[0].resultCap)
		assert.Equal(t,
			"https://api.whatsonchain.com/v1/bsv/main/address/1BitcoinEaterAddressDontSendf59kuE/unspent/all",
			providers[1].url)
		assert.Equal(t, 0, providers[1].resultCap)
	})

	t.Run("testnet switches BananaBlocks host, not path", func(t *testing.T) {
		providers := unspentProviders("myKamgwv8HY4eXvPVwvFsgZTXibD2gjGLk", true)

		require.Len(t, providers, 2)
		assert.Equal(t,
			"https://test.bananablocks.com/api/v1/bsv/main/address/myKamgwv8HY4eXvPVwvFsgZTXibD2gjGLk/unspent/all?limit=1000",
			providers[0].url)
		assert.Equal(t,
			"https://api.whatsonchain.com/v1/bsv/test/address/myKamgwv8HY4eXvPVwvFsgZTXibD2gjGLk/unspent/all",
			providers[1].url)
	})
}

func TestCheckComplete(t *testing.T) {
	t.Run("uncapped provider is always complete", func(t *testing.T) {
		assert.NoError(t, checkComplete([]byte(`{"result":[{},{},{}]}`), 0))
	})

	t.Run("under the cap is complete", func(t *testing.T) {
		assert.NoError(t, checkComplete([]byte(`{"result":[{},{}]}`), 3))
	})

	t.Run("a full page is rejected as possibly truncated", func(t *testing.T) {
		assert.ErrorIs(t, checkComplete([]byte(`{"result":[{},{},{}]}`), 3), errTruncated)
	})

	t.Run("unparseable body is an error", func(t *testing.T) {
		assert.Error(t, checkComplete([]byte(`not json`), 3))
	})
}
