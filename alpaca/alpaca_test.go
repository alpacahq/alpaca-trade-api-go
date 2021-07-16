package alpaca

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v2/marketdata"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// Copied from Gobroker v4.9.162 to test API conversion to backend struct
type CreateOrderRequest struct {
	AccountID     string           `json:"-"`
	ClientID      string           `json:"client_id"`
	OrderClass    string           `json:"order_class"`
	OrderID       *string          `json:"-"`
	ClientOrderID string           `json:"client_order_id"`
	AssetKey      string           `json:"symbol"`
	AssetID       string           `json:"-"`
	Qty           decimal.Decimal  `json:"qty"`
	Side          string           `json:"side"`
	Type          string           `json:"type"`
	TimeInForce   string           `json:"time_in_force"`
	LimitPrice    *decimal.Decimal `json:"limit_price"`
	StopPrice     *decimal.Decimal `json:"stop_price"`
	ExtendedHours bool             `json:"extended_hours"`
	Source        *string          `json:"source"`
	TakeProfit    *TakeProfit      `json:"take_profit"`
	StopLoss      *StopLoss        `json:"stop_loss"`
}

type AlpacaTestSuite struct {
	suite.Suite
}

func TestAlpacaTestSuite(t *testing.T) {
	suite.Run(t, new(AlpacaTestSuite))
}

func (s *AlpacaTestSuite) TestAlpaca() {
	// get account
	{
		// successful
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			account := Account{
				ID: "some_id",
			}
			return &http.Response{
				Body: genBody(account),
			}, nil
		}

		acct, err := GetAccount()
		assert.NoError(s.T(), err)
		assert.NotNil(s.T(), acct)
		assert.Equal(s.T(), "some_id", acct.ID)

		// api failure
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		acct, err = GetAccount()
		assert.Error(s.T(), err)
		assert.Nil(s.T(), acct)
	}

	// list positions
	{
		// successful
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			positions := []Position{
				{Symbol: "APCA"},
			}
			return &http.Response{
				Body: genBody(positions),
			}, nil
		}

		positions, err := ListPositions()
		assert.NoError(s.T(), err)
		assert.Len(s.T(), positions, 1)

		// api failure
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		positions, err = ListPositions()
		assert.Error(s.T(), err)
		assert.Nil(s.T(), positions)
	}

	// get aggregates
	{
		// successful
		aggregatesJSON := `{
			"ticker":"AAPL",
			"status":"OK",
			"adjusted":true,
			"queryCount":2,
			"resultsCount":2,
			"results":[
				{"v":52521891,"o":300.95,"c":288.08,"h":302.53,"l":286.13,"t":1582606800000,"n":1},
				{"v":46094168,"o":286.53,"c":292.69,"h":297.88,"l":286.5,"t":1582693200000,"n":1}
			]
		}`

		expectedAggregates := Aggregates{
			Ticker:       "AAPL",
			Status:       "OK",
			Adjusted:     true,
			QueryCount:   2,
			ResultsCount: 2,
			Results: []AggV2{
				{
					Volume:        52521891,
					Open:          300.95,
					Close:         288.08,
					High:          302.53,
					Low:           286.13,
					Timestamp:     1582606800000,
					NumberOfItems: 1,
				},
				{
					Volume:        46094168,
					Open:          286.53,
					Close:         292.69,
					High:          297.88,
					Low:           286.5,
					Timestamp:     1582693200000,
					NumberOfItems: 1,
				},
			},
		}
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{
				Body: ioutil.NopCloser(strings.NewReader(aggregatesJSON)),
			}, nil
		}

		actualAggregates, err := GetAggregates("AAPL", "minute", "2020-02-25", "2020-02-26")
		assert.NotNil(s.T(), actualAggregates)
		assert.NoError(s.T(), err)
		assert.EqualValues(s.T(), &expectedAggregates, actualAggregates)

		// api failure
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		actualAggregates, err = GetAggregates("AAPL", "minute", "2020-02-25", "2020-02-26")
		assert.Error(s.T(), err)
		assert.Nil(s.T(), actualAggregates)
	}
	// get last quote
	{
		// successful
		lastQuoteJSON := `{
			"status": "success",
			"symbol": "AAPL",
			"last": {
				"askprice":291.24,
				"asksize":1,
				"askexchange":2,
				"bidprice":291.76,
				"bidsize":1,
				"bidexchange":9,
				"timestamp":1582754386000
			}
		}`

		expectedLastQuote := LastQuoteResponse{
			Status: "success",
			Symbol: "AAPL",
			Last: LastQuote{
				AskPrice:    291.24,
				AskSize:     1,
				AskExchange: 2,
				BidPrice:    291.76,
				BidSize:     1,
				BidExchange: 9,
				Timestamp:   1582754386000,
			},
		}
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{
				Body: ioutil.NopCloser(strings.NewReader(lastQuoteJSON)),
			}, nil
		}

		actualLastQuote, err := GetLastQuote("AAPL")
		assert.NotNil(s.T(), actualLastQuote)
		assert.NoError(s.T(), err)
		assert.EqualValues(s.T(), &expectedLastQuote, actualLastQuote)

		// api failure
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		actualLastQuote, err = GetLastQuote("AAPL")
		assert.Error(s.T(), err)
		assert.Nil(s.T(), actualLastQuote)
	}

	// get last trade
	{
		// successful
		lastTradeJSON := `{
			"status": "success",
			"symbol": "AAPL",
			"last": {
				"price":290.614,
				"size":200,
				"exchange":2,
				"cond1":12,
				"cond2":1,
				"cond3":2,
				"cond4":3,
				"timestamp":1582756144000
			}
		}`
		expectedLastTrade := LastTradeResponse{
			Status: "success",
			Symbol: "AAPL",
			Last: LastTrade{
				Price:     290.614,
				Size:      200,
				Exchange:  2,
				Cond1:     12,
				Cond2:     1,
				Cond3:     2,
				Cond4:     3,
				Timestamp: 1582756144000,
			},
		}
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{
				Body: ioutil.NopCloser(strings.NewReader(lastTradeJSON)),
			}, nil
		}

		actualLastTrade, err := GetLastTrade("AAPL")
		assert.NotNil(s.T(), actualLastTrade)
		assert.NoError(s.T(), err)
		assert.EqualValues(s.T(), &expectedLastTrade, actualLastTrade)

		// api failure
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		actualLastTrade, err = GetLastTrade("AAPL")
		assert.Error(s.T(), err)
		assert.Nil(s.T(), actualLastTrade)
	}

	// get latest trade
	{
		// successful
		latestTradeJSON := `{
			"symbol": "AAPL",
			"trade": {
				"t": "2021-04-20T12:40:34.484136Z",
				"x": "J",
				"p": 134.7,
				"s": 20,
				"c": [
					"@",
					"T",
					"I"
				],
				"i": 32,
				"z": "C"
			}
		}`
		expectedLatestTrade := marketdata.Trade{
			ID:         32,
			Exchange:   "J",
			Price:      134.7,
			Size:       20,
			Timestamp:  time.Date(2021, 4, 20, 12, 40, 34, 484136000, time.UTC),
			Conditions: []string{"@", "T", "I"},
			Tape:       "C",
		}
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{
				Body: ioutil.NopCloser(strings.NewReader(latestTradeJSON)),
			}, nil
		}

		actualLatestTrade, err := GetLatestTrade("AAPL")
		require.NoError(s.T(), err)
		require.NotNil(s.T(), actualLatestTrade)
		assert.Equal(s.T(), expectedLatestTrade, *actualLatestTrade)

		// api failure
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		actualLatestTrade, err = GetLatestTrade("AAPL")
		assert.Error(s.T(), err)
		assert.Nil(s.T(), actualLatestTrade)
	}

	// get latest quote
	{
		// successful
		latestQuoteJSON := `{
				"symbol": "AAPL",
				"quote": {
					"t": "2021-04-20T13:01:57.822745906Z",
					"ax": "Q",
					"ap": 134.68,
					"as": 1,
					"bx": "K",
					"bp": 134.66,
					"bs": 29,
					"c": [
						"R"
					]
				}
			}`
		expectedLatestQuote := marketdata.Quote{
			BidExchange: "K",
			BidPrice:    134.66,
			BidSize:     29,
			AskExchange: "Q",
			AskPrice:    134.68,
			AskSize:     1,
			Timestamp:   time.Date(2021, 04, 20, 13, 1, 57, 822745906, time.UTC),
			Conditions:  []string{"R"},
		}
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{
				Body: ioutil.NopCloser(strings.NewReader(latestQuoteJSON)),
			}, nil
		}

		actualLatestQuote, err := GetLatestQuote("AAPL")
		require.NoError(s.T(), err)
		require.NotNil(s.T(), actualLatestQuote)
		assert.Equal(s.T(), expectedLatestQuote, *actualLatestQuote)

		// api failure
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		actualLatestQuote, err = GetLatestQuote("AAPL")
		assert.Error(s.T(), err)
		assert.Nil(s.T(), actualLatestQuote)
	}

	// get snapshot
	{
		// successful
		snapshotJSON := `{
			"symbol": "AAPL",
			"latestTrade": {
				"t": "2021-05-03T14:45:50.456Z",
				"x": "D",
				"p": 133.55,
				"s": 200,
				"c": [
					"@"
				],
				"i": 61462,
				"z": "C"
			},
			"latestQuote": {
				"t": "2021-05-03T14:45:50.532316972Z",
				"ax": "P",
				"ap": 133.55,
				"as": 7,
				"bx": "Q",
				"bp": 133.54,
				"bs": 9,
				"c": [
					"R"
				]
			},
			"minuteBar": {
				"t": "2021-05-03T14:44:00Z",
				"o": 133.485,
				"h": 133.4939,
				"l": 133.42,
				"c": 133.445,
				"v": 182818
			},
			"dailyBar": {
				"t": "2021-05-03T04:00:00Z",
				"o": 132.04,
				"h": 134.07,
				"l": 131.83,
				"c": 133.445,
				"v": 25094213
			},
			"prevDailyBar": {
				"t": "2021-04-30T04:00:00Z",
				"o": 131.82,
				"h": 133.56,
				"l": 131.065,
				"c": 131.46,
				"v": 109506363
			}
		}`
		expected := marketdata.Snapshot{
			LatestTrade: &marketdata.Trade{
				ID:         61462,
				Exchange:   "D",
				Price:      133.55,
				Size:       200,
				Timestamp:  time.Date(2021, 5, 3, 14, 45, 50, 456000000, time.UTC),
				Conditions: []string{"@"},
				Tape:       "C",
			},
			LatestQuote: &marketdata.Quote{
				BidExchange: "Q",
				BidPrice:    133.54,
				BidSize:     9,
				AskExchange: "P",
				AskPrice:    133.55,
				AskSize:     7,
				Timestamp:   time.Date(2021, 5, 3, 14, 45, 50, 532316972, time.UTC),
				Conditions:  []string{"R"},
			},
			MinuteBar: &marketdata.Bar{
				Open:      133.485,
				High:      133.4939,
				Low:       133.42,
				Close:     133.445,
				Volume:    182818,
				Timestamp: time.Date(2021, 5, 3, 14, 44, 0, 0, time.UTC),
			},
			DailyBar: &marketdata.Bar{
				Open:      132.04,
				High:      134.07,
				Low:       131.83,
				Close:     133.445,
				Volume:    25094213,
				Timestamp: time.Date(2021, 5, 3, 4, 0, 0, 0, time.UTC),
			},
			PrevDailyBar: &marketdata.Bar{
				Open:      131.82,
				High:      133.56,
				Low:       131.065,
				Close:     131.46,
				Volume:    109506363,
				Timestamp: time.Date(2021, 4, 30, 4, 0, 0, 0, time.UTC),
			},
		}
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{
				Body: ioutil.NopCloser(strings.NewReader(snapshotJSON)),
			}, nil
		}

		got, err := GetSnapshot("AAPL")
		require.NoError(s.T(), err)
		require.NotNil(s.T(), got)
		assert.Equal(s.T(), expected, *got)

		// api failure
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		got, err = GetSnapshot("AAPL")
		assert.Error(s.T(), err)
		assert.Nil(s.T(), got)
	}

	// get snapshots
	{
		// successful
		snapshotsJSON := `{
			"AAPL": {
				"latestTrade": {
					"t": "2021-05-03T14:48:06.563Z",
					"x": "D",
					"p": 133.4201,
					"s": 145,
					"c": [
						"@"
					],
					"i": 62700,
					"z": "C"
				},
				"latestQuote": {
					"t": "2021-05-03T14:48:07.257820915Z",
					"ax": "Q",
					"ap": 133.43,
					"as": 7,
					"bx": "Q",
					"bp": 133.42,
					"bs": 15,
					"c": [
						"R"
					]
				},
				"minuteBar": {
					"t": "2021-05-03T14:47:00Z",
					"o": 133.4401,
					"h": 133.48,
					"l": 133.37,
					"c": 133.42,
					"v": 207020,
					"n": 1234,
					"vw": 133.3987
				},
				"dailyBar": {
					"t": "2021-05-03T04:00:00Z",
					"o": 132.04,
					"h": 134.07,
					"l": 131.83,
					"c": 133.42,
					"v": 25846800,
					"n": 254678,
					"vw": 132.568
				},
				"prevDailyBar": {
					"t": "2021-04-30T04:00:00Z",
					"o": 131.82,
					"h": 133.56,
					"l": 131.065,
					"c": 131.46,
					"v": 109506363,
					"n": 1012323,
					"vw": 132.025
				}
			},
			"MSFT": {
				"latestTrade": {
					"t": "2021-05-03T14:48:06.36Z",
					"x": "D",
					"p": 253.8738,
					"s": 100,
					"c": [
						"@"
					],
					"i": 22973,
					"z": "C"
				},
				"latestQuote": {
					"t": "2021-05-03T14:48:07.243353456Z",
					"ax": "N",
					"ap": 253.89,
					"as": 2,
					"bx": "Q",
					"bp": 253.87,
					"bs": 2,
					"c": [
						"R"
					]
				},
				"minuteBar": {
					"t": "2021-05-03T14:47:00Z",
					"o": 253.78,
					"h": 253.869,
					"l": 253.78,
					"c": 253.855,
					"v": 25717,
					"n": 137,
					"vw": 253.823
				},
				"dailyBar": {
					"t": "2021-05-03T04:00:00Z",
					"o": 253.34,
					"h": 254.35,
					"l": 251.8,
					"c": 253.855,
					"v": 6100459,
					"n": 33453,
					"vw": 253.0534
				},
				"prevDailyBar": null
			},
			"INVALID": null
		}`
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{
				Body: ioutil.NopCloser(strings.NewReader(snapshotsJSON)),
			}, nil
		}

		got, err := GetSnapshots([]string{"AAPL", "MSFT", "INVALID"})
		require.NoError(s.T(), err)
		require.NotNil(s.T(), got)
		assert.Len(s.T(), got, 3)
		assert.Nil(s.T(), got["INVALID"])
		assert.EqualValues(s.T(), 7, got["AAPL"].LatestQuote.AskSize)
		assert.EqualValues(s.T(), 254678, got["AAPL"].DailyBar.TradeCount)
		assert.EqualValues(s.T(), 132.025, got["AAPL"].PrevDailyBar.VWAP)
		assert.EqualValues(s.T(), 6100459, got["MSFT"].DailyBar.Volume)
		assert.EqualValues(s.T(), 137, got["MSFT"].MinuteBar.TradeCount)
		assert.Nil(s.T(), got["MSFT"].PrevDailyBar)

		// api failure
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		got, err = GetSnapshots([]string{"AAPL", "CLDR"})
		assert.Error(s.T(), err)
		assert.Nil(s.T(), got)
	}

	// get clock
	{
		// successful
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			clock := Clock{
				Timestamp: time.Now(),
				IsOpen:    true,
				NextOpen:  time.Now(),
				NextClose: time.Now(),
			}
			return &http.Response{
				Body: genBody(clock),
			}, nil
		}

		clock, err := GetClock()
		assert.NoError(s.T(), err)
		assert.NotNil(s.T(), clock)
		assert.True(s.T(), clock.IsOpen)

		// api failure
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		clock, err = GetClock()
		assert.Error(s.T(), err)
		assert.Nil(s.T(), clock)
	}

	// get calendar
	{
		// successful
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			calendar := []CalendarDay{
				{
					Date:  "2018-01-01",
					Open:  time.Now().Format(time.RFC3339),
					Close: time.Now().Format(time.RFC3339),
				},
			}
			return &http.Response{
				Body: genBody(calendar),
			}, nil
		}

		start := "2018-01-01"
		end := "2018-01-02"

		calendar, err := GetCalendar(&start, &end)
		assert.NoError(s.T(), err)
		assert.Len(s.T(), calendar, 1)

		// api failure
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		calendar, err = GetCalendar(&start, &end)
		assert.Error(s.T(), err)
		assert.Nil(s.T(), calendar)
	}

	// list orders
	{
		// successful
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			orders := []Order{
				{
					ID: "some_id",
				},
			}
			return &http.Response{
				Body: genBody(orders),
			}, nil
		}

		status := "new"
		until := time.Now()
		limit := 1

		orders, err := ListOrders(&status, &until, &limit, nil)
		assert.NoError(s.T(), err)
		require.Len(s.T(), orders, 1)
		assert.Equal(s.T(), "some_id", orders[0].ID)

		// api failure
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		orders, err = ListOrders(&status, &until, &limit, nil)
		assert.Error(s.T(), err)
		assert.Nil(s.T(), orders)
	}

	// place order
	{
		// successful (w/ Qty)
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			por := PlaceOrderRequest{}
			if err := json.NewDecoder(req.Body).Decode(&por); err != nil {
				return nil, err
			}
			return &http.Response{
				Body: genBody(Order{
					Qty:         por.Qty,
					Notional:    por.Notional,
					Side:        por.Side,
					TimeInForce: por.TimeInForce,
					Type:        por.Type,
				}),
			}, nil
		}

		one := decimal.NewFromInt(1)
		req := PlaceOrderRequest{
			AccountID:   "some_id",
			Qty:         &one,
			Side:        Buy,
			TimeInForce: GTC,
			Type:        Limit,
		}

		order, err := PlaceOrder(req)
		assert.NoError(s.T(), err)
		assert.NotNil(s.T(), order)
		assert.Equal(s.T(), req.Qty, order.Qty)
		assert.Nil(s.T(), req.Notional)
		assert.Nil(s.T(), order.Notional)
		assert.Equal(s.T(), req.Type, order.Type)

		// successful (w/ Notional)
		req = PlaceOrderRequest{
			AccountID:   "some_id",
			Notional:    &one,
			Side:        Buy,
			TimeInForce: GTC,
			Type:        Limit,
		}

		order, err = PlaceOrder(req)
		assert.NoError(s.T(), err)
		assert.NotNil(s.T(), order)
		assert.Equal(s.T(), req.Notional, order.Notional)
		assert.Nil(s.T(), req.Qty)
		assert.Nil(s.T(), order.Qty)
		assert.Equal(s.T(), req.Type, order.Type)

		// api failure
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		order, err = PlaceOrder(req)
		assert.Error(s.T(), err)
		assert.Nil(s.T(), order)
	}

	// get order
	{
		// successful
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			order := Order{
				ID: "some_order_id",
			}
			return &http.Response{
				Body: genBody(order),
			}, nil
		}

		order, err := GetOrder("some_order_id")
		assert.NoError(s.T(), err)
		assert.NotNil(s.T(), order)

		// api failure
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		order, err = GetOrder("some_order_id")
		assert.Error(s.T(), err)
		assert.Nil(s.T(), order)
	}

	// get order by client_order_id
	{
		// successful
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			order := Order{
				ClientOrderID: "some_client_order_id",
			}
			return &http.Response{
				Body: genBody(order),
			}, nil
		}

		order, err := GetOrderByClientOrderID("some_client_order_id")
		assert.NoError(s.T(), err)
		assert.NotNil(s.T(), order)

		// api failure
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		order, err = GetOrderByClientOrderID("some_client_order_id")
		assert.Error(s.T(), err)
		assert.Nil(s.T(), order)
	}

	// cancel order
	{
		// successful
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{}, nil
		}

		assert.Nil(s.T(), CancelOrder("some_order_id"))

		// api failure
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		assert.NotNil(s.T(), CancelOrder("some_order_id"))
	}

	// list assets
	{
		// successful
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			assets := []Asset{
				{ID: "some_id"},
			}
			return &http.Response{
				Body: genBody(assets),
			}, nil
		}

		status := "active"

		assets, err := ListAssets(&status)
		assert.NoError(s.T(), err)
		require.Len(s.T(), assets, 1)
		assert.Equal(s.T(), "some_id", assets[0].ID)

		// api failure
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		assets, err = ListAssets(&status)
		assert.Error(s.T(), err)
		assert.Nil(s.T(), assets)
	}

	// get asset
	{
		// successful
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			asset := Asset{ID: "some_id"}
			return &http.Response{
				Body: genBody(asset),
			}, nil
		}

		asset, err := GetAsset("APCA")
		assert.NoError(s.T(), err)
		assert.NotNil(s.T(), asset)

		// api failure
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		asset, err = GetAsset("APCA")
		assert.Error(s.T(), err)
		assert.Nil(s.T(), asset)
	}

	// list bar lists
	{
		// successful
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			bars := []Bar{
				{
					Time:   1551157200,
					Open:   80.2,
					High:   80.86,
					Low:    80.02,
					Close:  80.51,
					Volume: 4283085,
				},
			}
			var barsMap = make(map[string][]Bar)
			barsMap["APCA"] = bars
			return &http.Response{
				Body: genBody(barsMap),
			}, nil
		}

		bars, err := ListBars([]string{"APCA"}, ListBarParams{Timeframe: "1D"})
		assert.NoError(s.T(), err)
		require.Len(s.T(), bars, 1)
		assert.Equal(s.T(), int64(1551157200), bars["APCA"][0].Time)

		// api failure
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		bars, err = ListBars([]string{"APCA"}, ListBarParams{Timeframe: "1D"})
		assert.Error(s.T(), err)
		assert.Nil(s.T(), bars)
	}

	// get bar list
	{
		// successful
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			bars := []Bar{
				{
					Time:   1551157200,
					Open:   80.2,
					High:   80.86,
					Low:    80.02,
					Close:  80.51,
					Volume: 4283085,
				},
			}
			var barsMap = make(map[string][]Bar)
			barsMap["APCA"] = bars
			return &http.Response{
				Body: genBody(barsMap),
			}, nil
		}

		bars, err := GetSymbolBars("APCA", ListBarParams{Timeframe: "1D"})
		assert.NoError(s.T(), err)
		assert.NotNil(s.T(), bars)

		// api failure
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		bars, err = GetSymbolBars("APCA", ListBarParams{Timeframe: "1D"})
		assert.Error(s.T(), err)
		assert.Nil(s.T(), bars)
	}

	// test verify
	{
		// 200
		resp := &http.Response{
			StatusCode: http.StatusOK,
		}

		assert.Nil(s.T(), verify(resp))

		// 500
		resp = &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       genBody(APIError{Code: 1010101, Message: "server is dead"}),
		}

		assert.NotNil(s.T(), verify(resp))
	}

	// test OTOCO Orders
	{
		do = func(c *Client, req *http.Request) (*http.Response, error) {
			or := CreateOrderRequest{}
			if err := json.NewDecoder(req.Body).Decode(&or); err != nil {
				return nil, err
			}
			return &http.Response{
				Body: genBody(Order{
					Qty:         &or.Qty,
					Side:        Side(or.Side),
					TimeInForce: TimeInForce(or.TimeInForce),
					Type:        OrderType(or.Type),
					Class:       string(or.OrderClass),
				}),
			}, nil
		}
		tpp := decimal.NewFromFloat(271.)
		spp := decimal.NewFromFloat(269.)
		tp := &TakeProfit{LimitPrice: &tpp}
		sl := &StopLoss{
			LimitPrice: nil,
			StopPrice:  &spp,
		}
		one := decimal.NewFromInt(0)
		req := PlaceOrderRequest{
			AccountID:   "some_id",
			Qty:         &one,
			Side:        Buy,
			TimeInForce: GTC,
			Type:        Limit,
			OrderClass:  Bracket,
			TakeProfit:  tp,
			StopLoss:    sl,
		}

		order, err := PlaceOrder(req)
		assert.NoError(s.T(), err)
		assert.NotNil(s.T(), order)
		assert.Equal(s.T(), "bracket", order.Class)
	}
}

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

func genBody(data interface{}) io.ReadCloser {
	buf, _ := json.Marshal(data)
	return nopCloser{bytes.NewBuffer(buf)}
}
