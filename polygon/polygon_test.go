package polygon

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type PolygonTestSuite struct {
	suite.Suite
}

func TestPolygonTestSuite(t *testing.T) {
	suite.Run(t, new(PolygonTestSuite))
}

func (s *PolygonTestSuite) TestPolygon() {
	// get historic aggregates
	{
		// successful
		get = func(u *url.URL) (*http.Response, error) {
			return &http.Response{
				Body: genBody([]byte(aggBody)),
			}, nil
		}

		now := time.Now()
		limit := 1

		resp, err := GetHistoricAggregates("APCA", Minute, &now, &now, &limit)
		assert.Nil(s.T(), err)
		assert.NotNil(s.T(), resp)

		// api failure
		get = func(u *url.URL) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		resp, err = GetHistoricAggregates("APCA", Minute, &now, &now, &limit)
		assert.NotNil(s.T(), err)
		assert.Nil(s.T(), resp)
	}

	// get historic trades
	{
		// successful
		get = func(u *url.URL) (*http.Response, error) {
			return &http.Response{
				Body: genBody([]byte(tradesBody)),
			}, nil
		}

		date := "2018-01-03"

		resp, err := GetHistoricTrades("APCA", date, nil)
		assert.Nil(s.T(), err)
		assert.NotNil(s.T(), resp)

		// api failure
		get = func(u *url.URL) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		resp, err = GetHistoricTrades("APCA", date, nil)
		assert.NotNil(s.T(), err)
		assert.Nil(s.T(), resp)
	}

	// get historic quotes
	{
		// successful
		get = func(u *url.URL) (*http.Response, error) {
			return &http.Response{
				Body: genBody([]byte(quotesBody)),
			}, nil
		}

		date := "2018-01-03"

		resp, err := GetHistoricQuotes("APCA", date)
		assert.Nil(s.T(), err)
		assert.NotNil(s.T(), resp)

		// api failure
		get = func(u *url.URL) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		resp, err = GetHistoricQuotes("APCA", date)
		assert.NotNil(s.T(), err)
		assert.Nil(s.T(), resp)
	}

	// get exchange data
	{
		// successful
		get = func(u *url.URL) (*http.Response, error) {
			return &http.Response{
				Body: genBody([]byte(exchangeBody)),
			}, nil
		}

		resp, err := GetStockExchanges()
		assert.Nil(s.T(), err)
		assert.NotNil(s.T(), resp)

		// api failure
		get = func(u *url.URL) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		resp, err = GetStockExchanges()
		assert.NotNil(s.T(), err)
		assert.Nil(s.T(), resp)
	}

	// get all tickers
	{
		// successful
		get = func(u *url.URL) (*http.Response, error) {
			return &http.Response{
				Body: genBody([]byte(allTickersBody)),
			}, nil
		}

		resp, err := GetAllTickers()
		assert.Nil(s.T(), err)
		assert.NotNil(s.T(), resp)

		// api failure
		get = func(u *url.URL) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		resp, err = GetAllTickers()
		assert.NotNil(s.T(), err)
		assert.Nil(s.T(), resp)
	}

	// get top 20 gainers
	{
		// successful
		get = func(u *url.URL) (*http.Response, error) {
			return &http.Response{
				Body: genBody([]byte(top20GainersLosersBody)),
			}, nil
		}

		resp, err := GetTop20Gainers()
		assert.Nil(s.T(), err)
		assert.NotNil(s.T(), resp)

		// api failure
		get = func(u *url.URL) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		resp, err = GetTop20Gainers()
		assert.NotNil(s.T(), err)
		assert.Nil(s.T(), resp)
	}

	// get top 20 losers
	{
		// successful
		get = func(u *url.URL) (*http.Response, error) {
			return &http.Response{
				Body: genBody([]byte(top20GainersLosersBody)),
			}, nil
		}

		resp, err := GetTop20Losers()
		assert.Nil(s.T(), err)
		assert.NotNil(s.T(), resp)

		// api failure
		get = func(u *url.URL) (*http.Response, error) {
			return &http.Response{}, fmt.Errorf("fail")
		}

		resp, err = GetTop20Losers()
		assert.NotNil(s.T(), err)
		assert.Nil(s.T(), resp)
	}

}

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

func genBody(buf []byte) io.ReadCloser {
	return nopCloser{bytes.NewBuffer(buf)}
}

const (
	aggBody = `{
		"symbol": "APCA",
		"aggType": "min",
		"map": {
		  "o": "open",
		  "c": "close",
		  "h": "high",
		  "l": "low",
		  "v": "volume",
		  "t": "timestamp"
		},
		"ticks": [
		  {
			"o": 47.53,
			"c": 47.53,
			"h": 47.53,
			"l": 47.53,
			"v": 16100,
			"t": 1199278800000
		  }
		]
	  }`
	quotesBody = `{
		"day": "2018-01-03",
		"map": {
		  "aE": "askexchange",
		  "aP": "askprice",
		  "aS": "asksize",
		  "bE": "bidexchange",
		  "bP": "bidprice",
		  "bS": "bidsize",
		  "c": "condition",
		  "t": "timestamp"
		},
		"msLatency": 7,
		"status": "success",
		"symbol": "APCA",
		"ticks": [
		  {
			"c": 0,
			"bE": "8",
			"aE": "11",
			"bP": 98.79,
			"aP": 98.89,
			"bS": 5,
			"aS": 1,
			"t": 1514938489451
		  }
		],
		"type": "quotes"
	  }`
	tradesBody = `{
		"day": "2018-01-03",
		"map": {
			"c1": "condition1",
			"c2": "condition2",
			"c3": "condition3",
			"c4": "condition4",
			"e": "exchange",
			"p": "price",
			"s": "size",
			"t": "timestamp"
		},
		"msLatency": 10,
		"status": "success",
		"symbol": "APCA",
		"ticks": [
			{
			"c1": 37,
			"c2": 12,
			"c3": 14,
			"c4": 0,
			"e": "8",
			"p": 98.82,
			"s": 61,
			"t": 1514938489451
			}
		],
		"type": "trades"
	}`
	exchangeBody = `[
		{
		  "id": 1,
		  "type": "exchange",
		  "market": "equities",
		  "mic": "XASE",
		  "name": "NYSE American (AMEX)",
		  "tape": "A"
		},
		{
		  "id": 2,
		  "type": "exchange",
		  "market": "equities",
		  "mic": "XBOS",
		  "name": "NASDAQ OMX BX",
		  "tape": "B"
		},
		{
		  "id": 15,
		  "type": "exchange",
		  "market": "equities",
		  "mic": "IEXG",
		  "name": "IEX",
		  "tape": "V"
		},
		{
		  "id": 16,
		  "type": "TRF",
		  "market": "equities",
		  "mic": "XCBO",
		  "name": "Chicago Board Options Exchange",
		  "tape": "W"
		}
	  ]`
	allTickersBody = `{
		  "status": "OK",
		  "tickers": [
			{
			  "ticker": "AAPL",
			  "day": {
				"c": 0.2907,
				"h": 0.2947,
				"l": 0.2901,
				"o": 0.2905,
				"v": 1432
			  },
			  "lastTrade": {
				"c1": 14,
				"c2": 12,
				"c3": 0,
				"c4": 0,
				"e": 12,
				"p": 172.17,
				"s": 50,
				"t": 1517529601006
			  },
			  "lastQuote": {
				"p": 120,
				"s": 5,
				"P": 121,
				"S": 3,
				"t": 1547787608999000000
			  },
			  "min": {
				"c": 0.2907,
				"h": 0.2947,
				"l": 0.2901,
				"o": 0.2905,
				"v": 1432
			  },
			  "prevDay": {
				"c": 0.2907,
				"h": 0.2947,
				"l": 0.2901,
				"o": 0.2905,
				"v": 1432
			  },
			  "todaysChange": 0.001,
			  "todaysChangePerc": 2.55,
			  "updated": 1547787608999
			}
		  ]
		}`
	top20GainersLosersBody = `{
		  "status": "OK",
		  "tickers": [
			{
			  "ticker": "AAPL",
			  "day": {
				"c": 0.2907,
				"h": 0.2947,
				"l": 0.2901,
				"o": 0.2905,
				"v": 1432
			  },
			  "lastTrade": {
				"c1": 14,
				"c2": 12,
				"c3": 0,
				"c4": 0,
				"e": 12,
				"p": 172.17,
				"s": 50,
				"t": 1517529601006
			  },
			  "lastQuote": {
				"p": 120,
				"s": 5,
				"P": 121,
				"S": 3,
				"t": 1547787608999000000
			  },
			  "min": {
				"c": 0.2907,
				"h": 0.2947,
				"l": 0.2901,
				"o": 0.2905,
				"v": 1432
			  },
			  "prevDay": {
				"c": 0.2907,
				"h": 0.2947,
				"l": 0.2901,
				"o": 0.2905,
				"v": 1432
			  },
			  "todaysChange": 0.001,
			  "todaysChangePerc": 2.55,
			  "updated": 1547787608999
			}
		  ]
		}`
)
