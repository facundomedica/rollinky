package app

import (
	"fmt"
	"math/big"
	"time"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	cometabci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/skip-mev/connect/v2/service/servers/oracle/types"

	connectabcitypes "github.com/skip-mev/connect/v2/abci/types"
	connecttypes "github.com/skip-mev/connect/v2/pkg/types"
	servicemetrics "github.com/skip-mev/connect/v2/service/metrics"
	oracletypes "github.com/skip-mev/connect/v2/x/oracle/types"
)

type RollkitHandler struct {
	// oracleClient oracleclient.OracleClient
	logger  log.Logger
	metrics servicemetrics.Metrics
	// ok is the oracle keeper that is used to write prices to state.
	ok connectabcitypes.OracleKeeper
}

func (h *RollkitHandler) PreBlocker(mm *module.Manager) sdk.PreBlocker {
	return func(ctx sdk.Context, req *cometabci.RequestFinalizeBlock) (_ *sdk.ResponsePreBlock, err error) {
		if req == nil {
			ctx.Logger().Error(
				"received nil RequestFinalizeBlock in oracle preblocker",
				"height", ctx.BlockHeight(),
			)

			return &sdk.ResponsePreBlock{}, fmt.Errorf("received nil RequestFinalizeBlock in oracle preblocker: height %d", ctx.BlockHeight())
		}

		// call module manager's PreBlocker first in case there is changes made on upgrades
		// that can modify state and lead to serialization/deserialization issues
		response, err := mm.PreBlock(ctx)
		if err != nil {
			return response, err
		}

		start := time.Now()
		prices := map[connecttypes.CurrencyPair]*big.Int{}
		defer func() {
			// only measure latency in Finalize
			if ctx.ExecMode() == sdk.ExecModeFinalize {
				latency := time.Since(start)
				h.logger.Debug(
					"finished executing the pre-block hook",
					"height", ctx.BlockHeight(),
					"latency (seconds)", latency.Seconds(),
				)
				connectabcitypes.RecordLatencyAndStatus(h.metrics, latency, err, servicemetrics.PreBlock)

				// record prices + ticker metrics per validator (only do so if there was no error writing the prices)
				if err == nil && prices != nil {
					// record price metrics
					h.recordPrices(prices)
				}
			}
		}()

		h.logger.Debug(
			"executing the pre-finalize block hook",
			"height", req.Height,
		)

		if len(req.Txs) == 0 {
			h.logger.Error(
				"no txs in block",
				"height", ctx.BlockHeight(),
			)
			return response, nil
		}

		rawPrices := &types.QueryPricesResponse{}
		if err := rawPrices.Unmarshal(req.Txs[0]); err != nil {
			h.logger.Error(
				"failed to unmarshal prices from txs",
				"height", ctx.BlockHeight(),
				"error", err,
			)
			return response, err
		}

		// Iterate over the prices and transform them into the correct format.
		for currencyPairID, priceString := range rawPrices.Prices {
			cp, err := connecttypes.CurrencyPairFromString(currencyPairID)
			if err != nil {
				return response, err
			}

			rawPrice, converted := new(big.Int).SetString(priceString, 10)
			if !converted {
				return response, fmt.Errorf("failed to convert price string to big.Int: %s", priceString)
			}

			prices[cp] = rawPrice
		}

		currencyPairs := h.ok.GetAllCurrencyPairs(ctx)
		for _, cp := range currencyPairs {
			price, ok := prices[cp]
			if !ok || price == nil {
				h.logger.Debug(
					"no price for currency pair",
					"currency_pair", cp.String(),
				)

				continue
			}

			if price.Sign() == -1 {
				h.logger.Error(
					"price is negative",
					"currency_pair", cp.String(),
					"price", price.String(),
				)

				continue
			}

			// Convert the price to a quote price and write it to state.
			quotePrice := oracletypes.QuotePrice{
				Price:          math.NewIntFromBigInt(price),
				BlockTimestamp: ctx.BlockHeader().Time,
				BlockHeight:    uint64(ctx.BlockHeight()), //nolint:gosec
			}

			if err := h.ok.SetPriceForCurrencyPair(ctx, cp, quotePrice); err != nil {
				h.logger.Error(
					"failed to set price for currency pair",
					"currency_pair", cp.String(),
					"quote_price", cp.String(),
					"err", err,
				)

				return nil, err
			}

			h.logger.Debug(
				"set price for currency pair",
				"currency_pair", cp.String(),
				"quote_price", quotePrice.Price.String(),
			)
		}

		return response, nil
	}
}

func (h *RollkitHandler) recordPrices(prices map[connecttypes.CurrencyPair]*big.Int) {
	for ticker, price := range prices {
		floatPrice, _ := price.Float64()
		h.metrics.ObservePriceForTicker(ticker, floatPrice)
	}
}
