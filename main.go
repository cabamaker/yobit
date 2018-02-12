/*
 * MIT License
 *
 * Copyright (c) 2018 Igor Konovalov
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

package main

import (
	"fmt"
	. "github.com/logrusorgru/aurora"
	"gopkg.in/alecthomas/kingpin.v2"
	"io/ioutil"
	"log"
	"os"
	"strings"
	cmc "github.com/ikonovalov/go-coinmarketcap"
)

var (
	defaultPair     = "btc_usd"
	defaultCurrency = "usd"

	app            = kingpin.New("yobit", "Yobit cryptocurrency exchange crafted client.").Version("0.3.2")
	appVerboseFlag = app.Flag("verbose", "Print additional information").Bool()

	cmdInit       = app.Command("init", "Initialize nonce and keys container")
	cmdInitSecret = cmdInit.Arg("secret", "API secret").Required().String()
	cmdInitKey    = cmdInit.Arg("key", "API key").Required().String()

	cmdMarkets      = app.Command("markets", "(m) Show all listed tickers on the Yobit").Alias("m")
	cmdInfoCurrency = cmdMarkets.Arg("cryptocurrency", "Show markets only for specified currency: btc, eth, usd and so on.").Default("").String()

	cmdTicker     = app.Command("ticker", "(tc) Command provides statistic data for the last 24 hours.").Alias("tc")
	cmdTickerPair = cmdTicker.Arg("pairs", "Listing ticker name. eth_btc, xem_usd, and so on.").Default(defaultPair).String()

	cmdDepth      = app.Command("depth", "(d) Command returns information about lists of active orders for selected pairs.").Alias("d")
	cmdDepthPair  = cmdDepth.Arg("pairs", "eth_btc, xem_usd and so on.").Default("defaultPair").String()
	cmdDepthLimit = cmdDepth.Arg("limit", "Depth output limit").Default("20").Int()

	cmdTrades      = app.Command("trades", "(tr) Command returns information about the last transactions of selected pairs.").Alias("tr")
	cmdTradesPair  = cmdTrades.Arg("pairs", "waves_btc, dash_usd and so on.").Default(defaultPair).String()
	cmdTradesLimit = cmdTrades.Arg("limit", "Trades output limit.").Default("100").Int()

	cmdWallets             = app.Command("wallets", "(w) Command returns information about user's balances and privileges of API-key as well as server time.").Alias("w")
	cmdWalletsBaseCurrency = cmdWallets.Arg("base-currency", "Base recalculated currency. Default: usd.").Default(defaultCurrency).String()

	cmdActiveOrders    = app.Command("active-orders", "(ao) Show active orders").Alias("ao")
	cmdActiveOrderPair = cmdActiveOrders.Arg("pair", "doge_usd...").Required().String()

	cmdOrderInfo   = app.Command("order", "(o) Detailed information about the chosen order").Alias("o")
	cmdOrderInfoId = cmdOrderInfo.Arg("id", "Order id").Required().String()

	cmdTradeHistory     = app.Command("trade-history", "(th) Trade history").Alias("th")
	cmdTradeHistoryPair = cmdTradeHistory.Arg("pair", "doge_usd...").Required().String()

	cmdBuy       = app.Command("buy", "(b) Buy on stock exchange").Alias("b")
	cmdBuyPair   = cmdBuy.Arg("pair", "Pair").Required().String()
	cmdBuyRate   = cmdBuy.Arg("rate", "Exchange rate for buying or selling").Required().Float64()
	cmdBuyAmount = cmdBuy.Arg("amount", "Exchange rate for buying or selling").Required().Float64()

	cmdSell       = app.Command("sell", "(s) Sell on stock exchange").Alias("s")
	cmdSellPair   = cmdSell.Arg("pair", "Pair").Required().String()
	cmdSellRate   = cmdSell.Arg("rate", "Exchange rate for buying or selling").Required().Float64()
	cmdSellAmount = cmdSell.Arg("amount", "Exchange rate for buying or selling").Required().Float64()

	cmdCancelOrder        = app.Command("cancel", "(c) Cancels the chosen order").Alias("c")
	cmdCancelOrderOrderId = cmdCancelOrder.Arg("order_id", "Order ID").Required().String()

	cmdCmc = app.Command("cmc", "Access to the CMC data")
	cmdCmcGlobals = cmdCmc.Command("global", "Show global markets").Default()
	cmdCmcMarkets = cmdCmc.Command("markets", "Show markets")
)

func main() {

	command := kingpin.MustParse(app.Parse(os.Args[1:]))

	// setup logging
	if !*appVerboseFlag {
		log.SetFlags(0)
		log.SetOutput(ioutil.Discard)
	}

	yobit := NewYobit()
	defer yobit.store.Release()

	switch command {
	case "init":
		{
			createCredentialFile(ApiCredential{Secret: *cmdInitSecret, Key: *cmdInitKey})
			createNonceFileIfNotExists()
			writeNonce([]byte("1"))
		}
	case "markets":
		{
			channel := make(chan InfoResponse)
			go yobit.Info(channel)
			infoResponse := <-channel
			printInfoRecords(infoResponse, *cmdInfoCurrency)
			fmt.Printf("\nTotal markets %d\n", len(infoResponse.Pairs))
		}
	case "ticker":
		{
			channel := make(chan TickerInfoResponse)
			go yobit.Tickers24([]string{strings.ToLower(*cmdTickerPair)}, channel)
			tickerResponse := <-channel

			for ticker, v := range tickerResponse.Tickers {
				printTicker(v, ticker)
			}
		}
	case "depth":
		{
			channel := make(chan DepthResponse)
			go yobit.DepthLimited(strings.ToLower(*cmdDepthPair), *cmdDepthLimit, channel)
			depthResponse := <-channel
			offers := depthResponse.Offers[*cmdDepthPair]
			printOffers(offers)

		}
	case "trades":
		{
			channel := make(chan TradesResponse)
			go yobit.TradesLimited(strings.ToLower(*cmdTradesPair), *cmdTradesLimit, channel)
			tradesResponse := <-channel
			for ticker, trades := range tradesResponse.Trades {
				fmt.Println(Bold(strings.ToUpper(ticker)))
				printTrades(trades)

			}
		}
	case "wallets":
		{
			channel := make(chan GetInfoResponse)
			go yobit.GetInfo(channel)
			getInfoRes := <-channel
			data := getInfoRes.Data
			funds := data.FundsIncludeOrders
			usdPairs := make([]string, 0, len(funds))
			for coin, volume := range funds {
				pair := fmt.Sprintf("%s_%s", coin, *cmdWalletsBaseCurrency)
				if volume > 0 && yobit.isMarketExists(pair) {
					usdPairs = append(usdPairs, pair)
				}
			}
			tickersChan := make(chan TickerInfoResponse)
			go yobit.Tickers24(usdPairs, tickersChan)
			tickerRs := <-tickersChan
			fundsAndTickers := struct {
				funds     map[string]float64
				freeFunds map[string]float64
				tickers   map[string]Ticker
			}{funds: data.FundsIncludeOrders, freeFunds: data.Funds, tickers: tickerRs.Tickers}
			printWallets(*cmdWalletsBaseCurrency, fundsAndTickers, data.ServerTime)
		}
	case "active-orders":
		{
			channel := make(chan ActiveOrdersResponse)
			go yobit.ActiveOrders(*cmdActiveOrderPair, channel)
			activeOrders := <-channel
			printActiveOrders(activeOrders)

		}
	case "order":
		{
			channel := make(chan OrderInfoResponse)
			go yobit.OrderInfo(*cmdOrderInfoId, channel)
			order := <-channel
			printOrderInfo(order.Orders)
		}
	case "trade-history":
		{
			channel := make(chan TradeHistoryResponse)
			go yobit.TradeHistory(*cmdTradeHistoryPair, channel)
			history := <-channel
			printTradeHistory(history)
		}
	case "buy":
		{
			channel := make(chan TradeResponse)
			go yobit.Trade(*cmdBuyPair, "buy", *cmdBuyRate, *cmdBuyAmount, channel)
			trade := <-channel
			printTradeResult(trade.Result)
		}
	case "sell":
		{
			channel := make(chan TradeResponse)
			go yobit.Trade(*cmdSellPair, "sell", *cmdSellRate, *cmdSellAmount, channel)
			trade := <-channel
			printTradeResult(trade.Result)
		}
	case "cancel":
		{
			channel := make(chan CancelOrderResponse)
			go yobit.CancelOrder(*cmdCancelOrderOrderId, channel)
			cancelResult := <-channel
			fmt.Printf("Order %d candeled\n", cancelResult.Result.OrderId)
		}
	case "cmc global":
		{
			channel := make(chan cmc.GlobalMarketData)
			go func() {
				marketInfo, err := cmc.GetMarketData()
				if err != nil {
					fatal(err)
				} else {
					channel <- marketInfo
				}
			}()
			marketInfo := <-channel
			printCmcGlobalMarket(marketInfo)
		}
	default:
		panic("Unknown command " + command)
	}

}
