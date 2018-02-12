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
	"github.com/olekukonko/tablewriter"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
	"sort"
	"github.com/ikonovalov/go-coinmarketcap"
	"github.com/leekchan/accounting"
)

var (
	bold []int = tablewriter.Colors{tablewriter.Bold}
	norm []int = tablewriter.Colors{0}
)

func sprintf64(v float64) string {
	return fmt.Sprintf("%8.8f", v)
}

func fatal(v ...interface{}) {
	fmt.Println(Red(Bold(fmt.Sprint(v))).String())
	os.Exit(1)
}

func printInfoRecords(infoResponse InfoResponse, currencyFilter string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Market", "Hidden", "Fee", "Min amount", "Min price", "Max price"})
	bold := tablewriter.Colors{tablewriter.Bold}
	norm := tablewriter.Colors{0}
	table.SetHeaderColor(bold, bold, bold, bold, bold, bold)
	table.SetColumnColor(bold, norm, norm, norm, norm, norm)

	currencyFilter = strings.ToUpper(currencyFilter)
	for name, desc := range infoResponse.Pairs {
		hidden := "NO"
		if desc.Hidden == 1 {
			hidden = "YES"
		}
		if currencyName := strings.ToUpper(name); currencyFilter == "" || strings.Contains(currencyName, currencyFilter) {
			table.Append([]string{
				currencyName,
				fmt.Sprintf("%s", hidden),
				fmt.Sprintf("%2.2f%%", desc.Fee),
				fmt.Sprintf("%8.8f", desc.MinAmount),
				fmt.Sprintf("%8.8f", desc.MinPrice),
				fmt.Sprintf("%8.8f", desc.MaxPrice),
			})
		}
	}
	table.Render()
}

func printWallets(groundCurrency string, fundsAndTickers struct {
	funds     map[string]float64
	freeFunds map[string]float64
	tickers   map[string]Ticker
}, updated int64) {
	// ground means supporting or recalculating currency. For example: "recalculate to an usd or a btc"
	// setup table
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{
		"#",
		"coin",
		"hold",
		"on order",
		fmt.Sprintf("%s RATE (24AVG)", groundCurrency),
		fmt.Sprintf("%s RATE (LAST)", groundCurrency),
		fmt.Sprintf("%s HOLD (24AVG)", groundCurrency),
		fmt.Sprintf("%s HOLD (LAST)", groundCurrency),
		"diff (abs)",
		"diff (%)",
		"coin",
	})
	table.SetHeaderColor(bold, bold, bold, bold, bold, bold, bold, bold, bold, bold, bold)
	table.SetColumnColor(bold, bold, norm, norm, norm, norm, norm, norm, norm, norm, bold)

	// determinate price multiplication indicator
	const one = float64(1)
	baseGroundPriceFunc := func(tickerName string) float64 {
		if tickerName == fmt.Sprintf("%s_%[1]s", groundCurrency) {
			return one
		} else {
			return fundsAndTickers.tickers[tickerName].Avg
		}
	}
	actualGroundPriceFunc := func(tickerName string) float64 {
		if tickerName == fmt.Sprintf("%s_%[1]s", groundCurrency) {
			return one
		} else {
			return fundsAndTickers.tickers[tickerName].Last
		}
	}
	onOrdersVisualFunction := func(ordered float64, volume float64) string {
		if ordered == 0 {
			return ""
		}
		if ordered-volume == 0 {
			return Red(fmt.Sprintf("%8.8f", ordered)).String()
		} else {
			return fmt.Sprintf("%8.8f", ordered)
		}
	}

	// define global accumulators
	var (
		baseGroundTotal   float64
		actualGroundTotal float64
		diffGroundTotal   float64 // diff between base and actual totals
	)

	// order coins by name
	coins := make([]string, 0, len(fundsAndTickers.funds))
	for c := range fundsAndTickers.funds {
		coins = append(coins, c)
	}
	sort.Strings(coins)

	var (
		winners = make([]string, 0, len(coins))
		losers  = make([]string, 0, len(coins))
	)

	rowCounter := 0

	for _, coin := range coins {
		volume := fundsAndTickers.funds[coin]
		onOrders := volume - fundsAndTickers.freeFunds[coin]
		if volume == 0 {
			continue
		}
		rowCounter++
		tickerName := fmt.Sprintf("%s_%s", coin, groundCurrency)

		baseGroundPrice := baseGroundPriceFunc(tickerName)
		baseGroundCoinPrice := volume * baseGroundPrice
		baseGroundTotal += baseGroundCoinPrice

		actualGroundPrice := actualGroundPriceFunc(tickerName)
		actualGroundCoinPrice := volume * actualGroundPrice
		actualGroundTotal += actualGroundCoinPrice

		diffGroundCoinPriceAbs := actualGroundCoinPrice - baseGroundCoinPrice
		diffGroundTotal += diffGroundCoinPriceAbs

		var diffGroundCoinPricePercent float64
		if diffGroundCoinPriceAbs != 0 {
			diffGroundCoinPricePercent = (actualGroundPrice - baseGroundPrice) / baseGroundPrice * float64(100)
		}

		var profitColor func(arg interface{}) Value
		if baseGroundPrice > actualGroundPrice {
			profitColor = Red
			losers = append(losers, coin)
		} else if baseGroundPrice == actualGroundPrice {
			profitColor = Gray
		} else {
			winners = append(winners, coin)
			profitColor = Green
		}

		coinUpperCase := strings.ToUpper(coin)
		table.Append([]string{
			fmt.Sprintf("%d", rowCounter),
			coinUpperCase,
			fmt.Sprintf("%.8f", volume),
			fmt.Sprintf("%s", onOrdersVisualFunction(onOrders, volume)),
			fmt.Sprintf("%.8f", baseGroundPrice),
			fmt.Sprintf("%.8f", profitColor(actualGroundPrice)),
			fmt.Sprintf("%.8f", baseGroundCoinPrice),
			fmt.Sprintf("%.8f", profitColor(actualGroundCoinPrice)),
			fmt.Sprintf("%+8.8f", profitColor(diffGroundCoinPriceAbs)),
			fmt.Sprintf("%+3.2f", profitColor(diffGroundCoinPricePercent)),
			coinUpperCase,
		})
	}

	table.SetFooter([]string{
		"",
		"",
		"",
		"",
		time.Unix(updated, 0).Format(time.Stamp),
		"Total cap",
		fmt.Sprintf("%8.2f", baseGroundTotal),
		fmt.Sprintf("%8.2f", actualGroundTotal),
		fmt.Sprintf("%+8.2f", diffGroundTotal),
		"",
		"",
	})

	fmt.Printf("%s Winners: %-3s %v\n", BgGreen(" "), fmt.Sprintf("%d", len(winners)), winners)
	fmt.Printf("%s Losers : %-3s %v\n", BgRed(" "), fmt.Sprintf("%d", len(losers)), losers)

	table.Render()
}

func printOffers(offers Offers) {
	var (
		asks    = offers.Asks
		bids    = offers.Bids
		asksLen = len(asks)
		bidsLen = len(bids)
		depth   = math.Max(float64(asksLen), float64(bidsLen))
	)
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{
		"#",
		"ask price",
		"ask quantity",
		"bid price",
		"bid quantity",
	})
	table.SetHeaderColor(bold, bold, bold, bold, bold)
	table.SetColumnColor(norm, norm, norm, norm, norm)

	appendOffer := func(row []string, offer Offer, wall bool) []string {
		qnt := sprintf64(offer.Quantity)
		if wall {
			qnt = Bold(qnt).String()
		}
		return append(row, sprintf64(offer.Price), qnt)
	}

	appendEmpty := func(row []string) []string {
		return append(row, "", "")
	}

	passingWall := func(index int, offers []Offer) bool { // TODO Need smarter algorithm!
		if index == len(offers)-1 {
			return false
		}
		offer := offers[index]
		nextOffer := offers[index+1]
		return offer.Quantity > nextOffer.Quantity*700.0
	}

	for i := 0; i < int(depth); i++ {
		row := append(make([]string, 0, 4), fmt.Sprintf("%d", i+1))

		if i < asksLen {
			ask := asks[i]
			row = appendOffer(row, ask, passingWall(i, asks))
		} else {
			row = appendEmpty(row)
		}
		if i < bidsLen {
			bid := bids[i]
			row = appendOffer(row, bid, passingWall(i, bids))
		} else {
			row = appendEmpty(row)
		}
		table.Append(row)
	}
	table.Render()
}

func lastHiGreen(first float64, second float64) (func(arg interface{}) Value) {
	if first > second {
		return Red
	} else if first == second {
		return Gray
	} else {
		return Green
	}
}

func printTicker(ticker Ticker, tickerName string) {
	spread := ticker.Sell - ticker.Buy
	spreadPercent := spread / ticker.Last * float64(100)
	updated := time.Unix(ticker.Updated, 0).Format(time.Stamp)

	table := tablewriter.NewWriter(os.Stdout)
	diffLastAvgPercent := (ticker.Last - ticker.Avg) / ticker.Avg * float64(100)
	diffLowAvgPercent := (ticker.Avg - ticker.Low) / ticker.Low * float64(100)

	table.SetHeader([]string{Bold(tickerName).String(), ""})
	table.SetColumnColor(bold, norm)
	table.Append([]string{"HIGH", sprintf64(ticker.High)})
	table.Append([]string{"LOW", sprintf64(ticker.Low)})
	table.Append([]string{
		"AVG", lastHiGreen(ticker.Low, ticker.Avg)(fmt.Sprintf("%8.8f\u00A0%+3.2f", ticker.Avg, diffLowAvgPercent)).String(),
	})
	table.Append([]string{
		"LAST",
		lastHiGreen(ticker.Avg, ticker.Last)(fmt.Sprintf("%8.8f\u00A0%+3.2f%%", ticker.Last, diffLastAvgPercent)).String(),
	})
	table.Append([]string{"BUY", sprintf64(ticker.Buy)})
	table.Append([]string{"SELL", sprintf64(ticker.Sell)})
	table.Append([]string{"SPREAD", lastHiGreen(0.5, spreadPercent)(fmt.Sprintf("%s\u00A0%+3.2f%%", sprintf64(spread), spreadPercent)).String()})
	table.Append([]string{"VOLUME", sprintf64(ticker.Vol)})
	table.Append([]string{"VOLUME CUR", sprintf64(ticker.VolCur)})

	fmt.Printf("%s\n", updated)
	table.Render()
}

func printTradeHistory(history TradeHistoryResponse) {
	//updated := time.Unix(ticker.Updated, 0).Format(time.Stamp)
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"tx id", "pair", "type", "rate", "amount", "time", "your order"})
	table.SetColumnColor(bold, bold, norm, norm, norm, norm, norm)
	directionMarker := func(dir string) string {
		dir = strings.ToUpper(dir)
		if dir == "BUY" {
			return BgGreen(dir).String()
		} else {
			return BgRed(dir).String()
		}
	}
	isYourOrderStr := func(flag uint8) string {
		if flag == 1 {
			return "YES"
		} else {
			return "NO"
		}
	}
	for tx, hOrder := range history.Orders {
		timestamp, _ := strconv.ParseInt(hOrder.Timestamp, 10, 64)
		timestampStr := time.Unix(timestamp, 0).Format(time.Stamp)
		table.Append([]string{
			tx,
			strings.ToUpper(hOrder.Pair),
			directionMarker(hOrder.Type),
			sprintf64(hOrder.Rate),
			sprintf64(hOrder.Amount),
			timestampStr,
			isYourOrderStr(hOrder.IsYourOrder),
		})
	}
	table.Render()
}

func printTrades(trades []Trade) {
	for _, trade := range trades {
		tm := time.Unix(trade.Timestamp, 0).Format(time.Stamp)
		Colored := BgGreen
		tradeDirection := "Buy "
		if trade.Type == "ask" {
			Colored = BgRed
			tradeDirection = "Sell"
		}

		fmt.Printf("%s %s Price[%.8f] Amount[%.8f] \u21D0 %d\n", tm, Bold(Colored(tradeDirection)), trade.Price, trade.Amount, trade.Tid)
	}
}

func printTradeResult(trade TradeResult) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{
		"OrderId",
		"Received",
		"Remains",
	})
	table.SetHeaderColor(bold, bold, bold)
	table.SetColumnColor(bold, norm, norm)
	table.Append([]string{
		fmt.Sprintf("%d", trade.OrderId),
		fmt.Sprintf("%8.8f", trade.Received),
		fmt.Sprintf("%8.8f", trade.Remains),
	})
	table.Render()
}

func printActiveOrders(activeOrders ActiveOrdersResponse) {
	for ordId, ord := range activeOrders.Orders {
		created, _ := strconv.ParseInt(ord.Created, 10, 64)
		fmt.Printf("%s ID[%s] %s amount: %.8f rate: %.8f\n",
			time.Unix(created, 0).Format(time.Stamp), ordId, strings.ToUpper(ord.Type), ord.Amount, ord.Rate)
	}
}

func printOrderInfo(orders map[string]OrderInfo) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{
		"orderid",
		"pair",
		"start amount",
		"amount",
		"fill",
		"rate",
		"created",
	})
	table.SetHeaderColor(bold, bold, bold, bold, bold, bold, bold)
	table.SetColumnColor(bold, bold, norm, norm, norm, norm, norm)
	for order, info := range orders {
		orderTime, _ := strconv.ParseInt(info.Created, 10, 64)
		fill := math.Abs(info.Amount-info.StartAmount) / info.StartAmount * float64(100)
		table.Append([]string{
			order,
			strings.ToUpper(info.Pair),
			sprintf64(info.StartAmount),
			sprintf64(info.Amount),
			fmt.Sprintf("%3.2f%%", fill),
			sprintf64(info.Rate),
			time.Unix(orderTime, 0).Format(time.Stamp),
		})
	}
	table.Render()
}

func printCmcGlobalMarket(data coinmarketcap.GlobalMarketData) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{
		"active assets",
		"active currencies",
		"active markets",
		"btc percentage",
		"total market cap usd",
		"total 24 volume usd",
	})
	table.SetHeaderColor(bold, bold, bold, bold, bold, bold)
	ac0p := accounting.Accounting{Symbol: "", Precision: 0}
	table.Append([]string{
		fmt.Sprintf("%d", data.ActiveAssets),
		fmt.Sprintf("%d", data.ActiveCurrencies),
		fmt.Sprintf("%d", data.ActiveMarkets),
		fmt.Sprintf("%3.2f%%", data.BitcoinPercentageOfMarketCap),
		ac0p.FormatMoney(data.TotalMarketCapUsd),
		ac0p.FormatMoney(data.Total24hVolumeUsd),
	})
	table.Render()
}
