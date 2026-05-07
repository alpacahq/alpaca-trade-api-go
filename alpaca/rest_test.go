package alpaca

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultDo(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "testkey", r.Header.Get("APCA-API-KEY-ID"))
		assert.Equal(t, "testsecret", r.Header.Get("APCA-API-SECRET-KEY"))
		assert.Equal(t, "/custompath", r.URL.Path)
		fmt.Fprint(w, "test body")
	}))
	c := NewClient(ClientOpts{
		APIKey:     "testkey",
		APISecret:  "testsecret",
		RetryDelay: time.Nanosecond,
		RetryLimit: 2,
		BaseURL:    ts.URL,
	})
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/custompath", nil)
	require.NoError(t, err)
	resp, err := defaultDo(c, req)
	require.NoError(t, err)
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "test body", string(b))
}

func TestDefaultDo_BrokerAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		notEmpty := assert.NotEmpty(t, authHeader)
		basicAuth := assert.True(t, strings.HasPrefix(authHeader, "Basic "), "%s is not basic auth", authHeader)
		if notEmpty && basicAuth {
			key := authHeader[len("Basic "):]
			b, err := base64.URLEncoding.DecodeString(key)
			if assert.NoError(t, err) {
				parts := strings.Split(string(b), ":")
				assert.Equal(t, "broker_key", parts[0])
				assert.Equal(t, "broker_secret", parts[1])
			}
		}
		fmt.Fprint(w, "test body")
	}))
	c := NewClient(ClientOpts{
		BrokerKey:    "broker_key",
		BrokerSecret: "broker_secret",
		BaseURL:      ts.URL,
	})
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/custompath", nil)
	require.NoError(t, err)
	resp, err := defaultDo(c, req)
	require.NoError(t, err)
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "test body", string(b))
}

func TestDefaultDo_SuccessfulRetries(t *testing.T) {
	i := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if i < 3 {
			i++
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, "success")
	}))
	c := NewClient(ClientOpts{
		RetryDelay: time.Nanosecond,
	})
	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	require.NoError(t, err)
	resp, err := defaultDo(c, req)
	require.NoError(t, err)
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "success", string(b))
}

func TestDefaultDo_TooManyRetries(t *testing.T) {
	i := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if i < 4 {
			i++
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, "success")
	}))
	c := NewClient(ClientOpts{
		RetryDelay: time.Nanosecond,
	})
	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	require.NoError(t, err)
	_, err = defaultDo(c, req)
	require.Error(t, err)
}

func TestDefaultDo_Error(t *testing.T) {
	resp := `{"code":1234567,"message":"custom error message","other_field":"x"}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, resp, http.StatusBadRequest)
	}))
	c := DefaultClient
	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	require.NoError(t, err)
	_, err = defaultDo(c, req)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Equal(t, 1234567, apiErr.Code)
	assert.Equal(t, "custom error message", apiErr.Message)
	assert.Equal(t, resp, apiErr.Body)
	assert.Equal(t, "custom error message (HTTP 400, Code 1234567)", apiErr.Error())
}

func TestGetAccount(t *testing.T) {
	c := DefaultClient

	// successful
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		account := Account{
			ID: "some_id",
		}
		return &http.Response{
			Body: genBody(account),
		}, nil
	}

	acct, err := c.GetAccount()
	require.NoError(t, err)
	assert.NotNil(t, acct)
	assert.Equal(t, "some_id", acct.ID)

	// api failure
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		return &http.Response{}, errors.New("fail")
	}

	_, err = c.GetAccount()
	require.Error(t, err)
}

func TestGetPositions(t *testing.T) {
	c := DefaultClient

	// successful
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		positions := []Position{
			{Symbol: "APCA"},
		}
		return &http.Response{
			Body: genBody(positions),
		}, nil
	}

	positions, err := c.GetPositions()
	require.NoError(t, err)
	assert.Len(t, positions, 1)

	// api failure
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		return &http.Response{}, errors.New("fail")
	}

	positions, err = c.GetPositions()
	require.Error(t, err)
	assert.Nil(t, positions)
}

func TestCancelPosition(t *testing.T) {
	c := DefaultClient
	order := &Order{
		ID:            "5aee8a3f-3ac8-42e0-b3e6-ed5cfdf85864",
		ClientOrderID: "0571ce61-bf65-4f0c-b3de-6f42de628422",
		Symbol:        "AAPL",
	}
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v2/positions/AAPL", req.URL.Path)
		assert.Equal(t, http.MethodDelete, req.Method)
		assert.Equal(t, "0.12345678", req.URL.Query().Get("qty"))
		return &http.Response{
			Body: genBody(order),
		}, nil
	}
	got, err := c.ClosePosition("AAPL", ClosePositionRequest{
		Qty: decimal.RequireFromString("0.12345678"),
	})
	require.NoError(t, err)
	assert.Equal(t, order.ID, got.ID)
	assert.Equal(t, order.ClientOrderID, got.ClientOrderID)
	assert.Equal(t, "AAPL", got.Symbol)
}

func TestCancelAllPositions(t *testing.T) {
	c := DefaultClient

	closeAllPositionsResponse := []closeAllPositionsResponse{
		{
			Symbol: "AAPL",
			Status: 200,
			Body:   json.RawMessage(`{"id":"0571ce61-bf65-4f0c-b3de-6f42ce628422", "symbol": "AAPL"}`),
		},
		{
			Symbol: "TSLA",
			Status: 422,
			Body:   json.RawMessage(`{"code": 42210000, "message": "error"}`),
		},
	}
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v2/positions", req.URL.Path)
		assert.Equal(t, http.MethodDelete, req.Method)
		assert.Equal(t, "true", req.URL.Query().Get("cancel_orders"))
		return &http.Response{
			Body: genBody(closeAllPositionsResponse),
		}, nil
	}
	gotOrders, err := c.CloseAllPositions(CloseAllPositionsRequest{
		CancelOrders: true,
	})
	require.Error(t, err)
	assert.Len(t, gotOrders, 1)
	assert.Equal(t, "AAPL", gotOrders[0].Symbol)
}

func TestGetClock(t *testing.T) {
	c := DefaultClient
	// successful
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
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

	clock, err := c.GetClock()
	require.NoError(t, err)
	assert.NotNil(t, clock)
	assert.True(t, clock.IsOpen)

	// api failure
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		return &http.Response{}, errors.New("fail")
	}

	_, err = c.GetClock()
	require.Error(t, err)
}

func TestGetCalendar(t *testing.T) {
	c := DefaultClient
	// successful
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "2018-01-01", req.URL.Query().Get("start"))
		assert.Equal(t, "2018-01-02", req.URL.Query().Get("end"))
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

	calendar, err := c.GetCalendar(GetCalendarRequest{
		Start: time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2018, 1, 2, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	assert.Len(t, calendar, 1)

	// api failure
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		return &http.Response{}, errors.New("fail")
	}

	calendar, err = c.GetCalendar(GetCalendarRequest{})
	require.Error(t, err)
	assert.Nil(t, calendar)
}

func TestGetOrders_EmptyRequest(t *testing.T) {
	c := DefaultClient
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v2/orders", req.URL.Path)
		assert.Equal(t, "", req.URL.Query().Get("status"))
		assert.Equal(t, "", req.URL.Query().Get("after"))
		assert.Equal(t, "", req.URL.Query().Get("until"))
		assert.Equal(t, "", req.URL.Query().Get("limit"))
		assert.Equal(t, "", req.URL.Query().Get("direction"))
		assert.Equal(t, "false", req.URL.Query().Get("nested"))
		assert.Equal(t, "", req.URL.Query().Get("symbols"))
		assert.Equal(t, "", req.URL.Query().Get("side"))

		orders := []Order{
			{
				ID: "some_id",
			},
		}
		return &http.Response{
			Body: genBody(orders),
		}, nil
	}

	req := GetOrdersRequest{}

	orders, err := c.GetOrders(req)
	require.NoError(t, err)
	require.Len(t, orders, 1)
	assert.Equal(t, "some_id", orders[0].ID)
}

func TestGetOrders(t *testing.T) {
	c := DefaultClient
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v2/orders", req.URL.Path)
		assert.Equal(t, "all", req.URL.Query().Get("status"))
		assert.Equal(t, "2021-04-03T00:00:00Z", req.URL.Query().Get("after"))
		assert.Equal(t, "2021-04-04T05:00:00Z", req.URL.Query().Get("until"))
		assert.Equal(t, "2", req.URL.Query().Get("limit"))
		assert.Equal(t, "asc", req.URL.Query().Get("direction"))
		assert.Equal(t, "true", req.URL.Query().Get("nested"))
		assert.Equal(t, "AAPL,TSLA", req.URL.Query().Get("symbols"))
		assert.Equal(t, "buy", req.URL.Query().Get("side"))

		orders := []Order{
			{
				ID: "some_id",
			},
		}
		return &http.Response{
			Body: genBody(orders),
		}, nil
	}
	req := GetOrdersRequest{
		Status:    "all",
		After:     time.Date(2021, 4, 3, 0, 0, 0, 0, time.UTC),
		Until:     time.Date(2021, 4, 4, 5, 0, 0, 0, time.UTC),
		Limit:     2,
		Direction: "asc",
		Nested:    true,
		Symbols:   []string{"AAPL", "TSLA"},
		Side:      "buy",
	}

	orders, err := c.GetOrders(req)
	require.NoError(t, err)
	require.Len(t, orders, 1)
	assert.Equal(t, "some_id", orders[0].ID)

	// api failure
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		return &http.Response{}, errors.New("fail")
	}

	orders, err = c.GetOrders(req)
	require.Error(t, err)
	assert.Nil(t, orders)
}

func TestPlaceOrder(t *testing.T) {
	c := DefaultClient
	// successful (w/ Qty)
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		por := PlaceOrderRequest{}
		if err := json.NewDecoder(req.Body).Decode(&por); err != nil {
			return nil, err
		}
		order := Order{
			Qty:         por.Qty,
			Notional:    por.Notional,
			Side:        por.Side,
			TimeInForce: por.TimeInForce,
			Type:        por.Type,
		}
		if por.PositionIntent != "" {
			order.PositionIntent = por.PositionIntent
		} else {
			// default to BuyToOpen/SellToOpen if not specified (for testing)
			if por.Side == Buy {
				order.PositionIntent = BuyToOpen
			} else {
				order.PositionIntent = SellToOpen
			}
		}
		return &http.Response{
			Body: genBody(order),
		}, nil
	}

	one := decimal.NewFromInt(1)
	req := PlaceOrderRequest{
		Qty:         &one,
		Side:        Buy,
		TimeInForce: GTC,
		Type:        Limit,
	}

	order, err := c.PlaceOrder(req)
	require.NoError(t, err)
	assert.NotNil(t, order)
	assert.Equal(t, req.Qty, order.Qty)
	assert.Nil(t, req.Notional)
	assert.Nil(t, order.Notional)
	assert.Equal(t, req.Type, order.Type)
	assert.Equal(t, BuyToOpen, order.PositionIntent)

	// successful (w/ Notional)
	req = PlaceOrderRequest{
		Notional:    &one,
		Side:        Buy,
		TimeInForce: GTC,
		Type:        Limit,
	}

	order, err = c.PlaceOrder(req)
	require.NoError(t, err)
	assert.NotNil(t, order)
	assert.Equal(t, req.Notional, order.Notional)
	assert.Nil(t, req.Qty)
	assert.Nil(t, order.Qty)
	assert.Equal(t, req.Type, order.Type)
	assert.Equal(t, BuyToOpen, order.PositionIntent)

	// successful (w/ Qty,PositionIntent)
	req = PlaceOrderRequest{
		Qty:            &one,
		Side:           Sell,
		TimeInForce:    GTC,
		Type:           Limit,
		PositionIntent: SellToClose,
	}

	order, err = c.PlaceOrder(req)
	require.NoError(t, err)
	assert.NotNil(t, order)
	assert.NotNil(t, req.Qty)
	assert.NotNil(t, order.Qty)
	assert.Equal(t, req.Type, order.Type)
	assert.Equal(t, SellToClose, order.PositionIntent)

	// api failure
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		return &http.Response{}, errors.New("fail")
	}

	_, err = c.PlaceOrder(req)
	require.Error(t, err)
}

func deciP(s string) *decimal.Decimal {
	d := decimal.RequireFromString(s)
	return &d
}

func TestPlaceMLegOrder(t *testing.T) {
	c := DefaultClient
	// mock response
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		por := PlaceOrderRequest{}
		if err := json.NewDecoder(req.Body).Decode(&por); err != nil {
			return nil, err
		}
		return &http.Response{
			Body: genBody(Order{
				Qty:         por.Qty,
				TimeInForce: por.TimeInForce,
				Type:        por.Type,
				OrderClass:  por.OrderClass,
				Legs: []Order{
					{
						Symbol:         por.Legs[0].Symbol,
						Side:           por.Legs[0].Side,
						PositionIntent: por.Legs[0].PositionIntent,
						Qty:            deciP("5"), // 5*1
					},
					{
						Symbol:         por.Legs[1].Symbol,
						Side:           por.Legs[1].Side,
						PositionIntent: por.Legs[1].PositionIntent,
						Qty:            deciP("10"), // 5*2
					},
					{
						Symbol:         por.Legs[2].Symbol,
						Side:           por.Legs[2].Side,
						PositionIntent: por.Legs[2].PositionIntent,
						Qty:            deciP("5"), // 5*1
					},
					{
						Symbol:         por.Legs[3].Symbol,
						Side:           por.Legs[3].Side,
						PositionIntent: por.Legs[3].PositionIntent,
						Qty:            deciP("10"), // 5*2
					},
				},
			}),
		}, nil
	}

	// simulate an iron condor
	qty := decimal.NewFromInt(5)
	req := PlaceOrderRequest{
		Qty:         &qty,
		TimeInForce: Day,
		Type:        Market,
		OrderClass:  MLeg,
		Legs: []Leg{
			{
				Symbol:         "AAPL241220P00245000", // strike A
				Side:           Buy,
				PositionIntent: BuyToOpen,
				RatioQty:       decimal.NewFromInt(1),
			},
			{
				Symbol:         "AAPL241220P00250000", // strike B
				Side:           Sell,
				PositionIntent: SellToOpen,
				RatioQty:       decimal.NewFromInt(2),
			},
			{
				Symbol:         "AAPL241220C00255000", // strike C
				Side:           Buy,
				PositionIntent: BuyToOpen,
				RatioQty:       decimal.NewFromInt(1),
			},
			{
				Symbol:         "AAPL241220C00260000", // strike D
				Side:           Sell,
				PositionIntent: SellToOpen,
				RatioQty:       decimal.NewFromInt(2),
			},
		},
	}

	order, err := c.PlaceOrder(req)
	require.NoError(t, err)
	assert.NotNil(t, order)
	assert.Equal(t, req.Qty, order.Qty)
	assert.Equal(t, req.TimeInForce, order.TimeInForce)
	assert.Equal(t, req.Type, order.Type)
	assert.Equal(t, req.OrderClass, order.OrderClass)

	// verify legs
	require.Len(t, order.Legs, len(req.Legs))
	for i := range req.Legs {
		workingQty := req.Legs[i].RatioQty.Mul(*req.Qty).String()
		assert.Equal(t, req.Legs[i].Symbol, order.Legs[i].Symbol)
		assert.Equal(t, req.Legs[i].Side, order.Legs[i].Side)
		assert.Equal(t, req.Legs[i].PositionIntent, order.Legs[i].PositionIntent)
		assert.Equal(t, workingQty, order.Legs[i].Qty.String())
	}
}

func TestGetOrder(t *testing.T) {
	c := DefaultClient
	// successful
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		order := Order{
			ID: "some_order_id",
		}
		return &http.Response{
			Body: genBody(order),
		}, nil
	}

	order, err := c.GetOrder("some_order_id")
	require.NoError(t, err)
	assert.NotNil(t, order)

	// api failure
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		return &http.Response{}, errors.New("fail")
	}

	_, err = c.GetOrder("some_order_id")
	require.Error(t, err)
}

func TestGetOrderByClientOrderId(t *testing.T) {
	c := DefaultClient
	// successful
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		order := Order{
			ClientOrderID: "some_client_order_id",
		}
		return &http.Response{
			Body: genBody(order),
		}, nil
	}

	order, err := c.GetOrderByClientOrderID("some_client_order_id")
	require.NoError(t, err)
	assert.NotNil(t, order)

	// api failure
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		return &http.Response{}, errors.New("fail")
	}

	_, err = c.GetOrderByClientOrderID("some_client_order_id")
	require.Error(t, err)
}

func TestClient_GetAnnouncements(t *testing.T) {
	c := DefaultClient
	// successful
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v2/corporate_actions/announcements", req.URL.Path)
		assert.Equal(t, http.MethodGet, req.Method)
		assert.Equal(t, "AAPL", req.URL.Query().Get("symbol"))
		assert.Equal(t, "some_cusip", req.URL.Query().Get("cusip"))
		assert.Equal(t, "declaration_date", req.URL.Query().Get("date_type"))
		assert.Equal(t, "Dividend,Merger", req.URL.Query().Get("ca_types"))
		assert.Equal(t, "2020-01-01", req.URL.Query().Get("since"))
		assert.Equal(t, "2020-01-02", req.URL.Query().Get("until"))

		announcements := []Announcement{
			{
				ID: "some_id",
			},
		}
		return &http.Response{
			Body: genBody(announcements),
		}, nil
	}

	announcements, err := c.GetAnnouncements(GetAnnouncementsRequest{
		CATypes:  []string{"Dividend", "Merger"},
		Since:    time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		Until:    time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC),
		Symbol:   "AAPL",
		Cusip:    "some_cusip",
		DateType: DeclarationDate,
	})
	require.NoError(t, err)
	require.Len(t, announcements, 1)
}

func TestClient_GetAnnouncement(t *testing.T) {
	c := DefaultClient
	// successful
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v2/corporate_actions/announcements/123", req.URL.Path)
		assert.Equal(t, http.MethodGet, req.Method)

		announcement := Announcement{
			ID: "some_id",
		}

		return &http.Response{
			Body: genBody(announcement),
		}, nil
	}

	announcement, err := c.GetAnnouncement("123")
	require.NoError(t, err)
	require.NotNil(t, announcement)
}

func TestClient_GetWatchlists(t *testing.T) {
	c := DefaultClient
	// successful
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v2/watchlists", req.URL.Path)
		assert.Equal(t, http.MethodGet, req.Method)

		watchlists := []Watchlist{
			{
				AccountID: "123",
				ID:        "some_id",
				Name:      "testname",
				Assets: []Asset{
					{
						ID:       "some_id",
						Name:     "AAPL",
						Exchange: "NASDAQ",
					},
				},
			},
		}

		return &http.Response{
			Body: genBody(watchlists),
		}, nil
	}

	watchlists, err := c.GetWatchlists()
	require.NoError(t, err)
	require.Len(t, watchlists, 1)
}

func TestClient_CreateWatchlist(t *testing.T) {
	c := DefaultClient
	// successful
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v2/watchlists", req.URL.Path)
		assert.Equal(t, "POST", req.Method)

		watchlist := Watchlist{
			AccountID: "123",
			ID:        "some_id",
			Name:      "testname",
			Assets: []Asset{
				{
					ID:       "some_id",
					Name:     "AAPL",
					Exchange: "NASDAQ",
				},
			},
		}

		return &http.Response{
			Body: genBody(watchlist),
		}, nil
	}

	watchlist, err := c.CreateWatchlist(CreateWatchlistRequest{
		Name:    "testname",
		Symbols: []string{"AAPL"},
	})
	require.NoError(t, err)
	require.NotNil(t, watchlist)
	require.Equal(t, "testname", watchlist.Name)
	require.Len(t, watchlist.Assets, 1)
	require.Equal(t, "AAPL", watchlist.Assets[0].Name)
	require.Equal(t, "NASDAQ", watchlist.Assets[0].Exchange)
}

func TestClient_GetWatchlist(t *testing.T) {
	c := DefaultClient
	// successful
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v2/watchlists/123", req.URL.Path)
		assert.Equal(t, http.MethodGet, req.Method)

		watchlist := Watchlist{
			AccountID: "123",
			ID:        "some_id",
			Name:      "testname",
			Assets: []Asset{
				{
					ID:       "some_id",
					Name:     "AAPL",
					Exchange: "NASDAQ",
				},
			},
		}

		return &http.Response{
			Body: genBody(watchlist),
		}, nil
	}

	watchlist, err := c.GetWatchlist("123")
	require.NoError(t, err)
	require.NotNil(t, watchlist)
	require.Equal(t, "testname", watchlist.Name)
	require.Len(t, watchlist.Assets, 1)
	require.Equal(t, "AAPL", watchlist.Assets[0].Name)
	require.Equal(t, "NASDAQ", watchlist.Assets[0].Exchange)
}

func TestClient_UpdateWatchlist(t *testing.T) {
	c := DefaultClient
	// successful
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v2/watchlists/123", req.URL.Path)
		assert.Equal(t, "PUT", req.Method)

		watchlist := Watchlist{
			AccountID: "123",
			ID:        "some_id",
			Name:      "testname",
			Assets: []Asset{
				{
					ID:       "some_id",
					Name:     "AAPL",
					Exchange: "NASDAQ",
				},
			},
		}

		return &http.Response{
			Body: genBody(watchlist),
		}, nil
	}

	watchlist, err := c.UpdateWatchlist("123", UpdateWatchlistRequest{
		Name:    "testname",
		Symbols: []string{"AAPL"},
	})
	require.NoError(t, err)
	require.NotNil(t, watchlist)
	require.Equal(t, "testname", watchlist.Name)
	require.Len(t, watchlist.Assets, 1)
	require.Equal(t, "AAPL", watchlist.Assets[0].Name)
	require.Equal(t, "NASDAQ", watchlist.Assets[0].Exchange)
}

func TestClient_DeleteWatchlist(t *testing.T) {
	c := DefaultClient
	// successful
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v2/watchlists/123", req.URL.Path)
		assert.Equal(t, "DELETE", req.Method)

		return &http.Response{
			Body: genBody(nil),
		}, nil
	}

	err := c.DeleteWatchlist("123")
	require.NoError(t, err)
}

func TestClient_AddSymbolToWatchlist(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		c := DefaultClient
		// successful
		c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
			assert.Equal(t, "/v2/watchlists/123", req.URL.Path)
			assert.Equal(t, "POST", req.Method)

			watchlist := Watchlist{
				AccountID: "123",
				ID:        "some_id",
				Name:      "testname",
				Assets: []Asset{
					{
						ID:       "some_id",
						Name:     "AAPL",
						Exchange: "NASDAQ",
					},
				},
			}

			return &http.Response{
				Body: genBody(watchlist),
			}, nil
		}

		watchlist, err := c.AddSymbolToWatchlist("123", AddSymbolToWatchlistRequest{
			Symbol: "AAPL",
		})
		require.NoError(t, err)
		require.NotNil(t, watchlist)
		require.Equal(t, "testname", watchlist.Name)
		require.Len(t, watchlist.Assets, 1)
		require.Equal(t, "AAPL", watchlist.Assets[0].Name)
		require.Equal(t, "NASDAQ", watchlist.Assets[0].Exchange)
	})

	t.Run("error: symbol not found", func(t *testing.T) {
		c := DefaultClient
		// successful
		c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
			assert.Equal(t, "/v2/watchlists/123", req.URL.Path)
			assert.Equal(t, "POST", req.Method)

			return &http.Response{
				Body:       genBody(nil),
				StatusCode: http.StatusBadRequest,
			}, nil
		}

		_, err := c.AddSymbolToWatchlist("123", AddSymbolToWatchlistRequest{})
		require.Error(t, err)
		require.Equal(t, ErrSymbolMissing, err)
	})
}

func TestClient_RemoveSymbolFromWatchlist(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		c := DefaultClient
		// successful
		c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
			assert.Equal(t, "/v2/watchlists/123/AAPL", req.URL.Path)
			assert.Equal(t, "DELETE", req.Method)

			return &http.Response{
				Body: genBody(nil),
			}, nil
		}

		err := c.RemoveSymbolFromWatchlist("123", RemoveSymbolFromWatchlistRequest{
			Symbol: "AAPL",
		})
		require.NoError(t, err)
	})

	t.Run("error: symbol is required", func(t *testing.T) {
		c := DefaultClient
		// successful
		c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
			assert.Equal(t, "/v2/watchlists/123/AAPL", req.URL.Path)
			assert.Equal(t, "DELETE", req.Method)

			return &http.Response{
				Body:       genBody(nil),
				StatusCode: http.StatusBadRequest,
			}, errors.New("symbol is required")
		}

		err := c.RemoveSymbolFromWatchlist("123", RemoveSymbolFromWatchlistRequest{})
		require.Error(t, err)
		require.Equal(t, ErrSymbolMissing, err)
	})
}

func TestCancelOrder(t *testing.T) {
	c := DefaultClient
	// successful
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		return &http.Response{}, nil
	}

	require.NoError(t, c.CancelOrder("some_order_id"))

	// api failure
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		return &http.Response{}, errors.New("fail")
	}

	assert.Error(t, c.CancelOrder("some_order_id"))
}

func TestGetAssets(t *testing.T) {
	c := DefaultClient
	// successful
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "active", req.URL.Query().Get("status"))
		assets := []Asset{
			{ID: "some_id"},
		}
		return &http.Response{
			Body: genBody(assets),
		}, nil
	}

	assets, err := c.GetAssets(GetAssetsRequest{
		Status: "active",
	})
	require.NoError(t, err)
	require.Len(t, assets, 1)
	assert.Equal(t, "some_id", assets[0].ID)

	// api failure
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		return &http.Response{}, errors.New("fail")
	}

	_, err = c.GetAssets(GetAssetsRequest{})
	require.Error(t, err)
}

func TestGetAsset(t *testing.T) {
	c := DefaultClient
	// successful
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		asset := Asset{ID: "some_id"}
		return &http.Response{
			Body: genBody(asset),
		}, nil
	}

	asset, err := c.GetAsset("APCA")
	require.NoError(t, err)
	assert.NotNil(t, asset)

	// api failure
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		return &http.Response{}, errors.New("fail")
	}

	asset, err = c.GetAsset("APCA")
	require.Error(t, err)
	assert.Nil(t, asset)
}

func TestGetAssetFromJSON(t *testing.T) {
	c := DefaultClient

	assetJSON := `{
			"id": "904837e3-3b76-47ec-b432-046db621571b",
			"class": "us_equity",
			"exchange": "NASDAQ",
			"symbol": "APCA",
			"status": "active",
			"tradable": true,
			"marginable": true,
			"shortable": true,
			"easy_to_borrow": true,
			"fractionable": true
		}`

	// successful
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(assetJSON)),
		}, nil
	}

	asset, err := c.GetAsset("APCA")
	require.NoError(t, err)
	assert.Equal(t, USEquity, asset.Class)
	assert.True(t, asset.Fractionable)
	assert.NotNil(t, asset)

	// api failure
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		return &http.Response{}, errors.New("fail")
	}

	asset, err = c.GetAsset("APCA")
	require.Error(t, err)
	assert.Nil(t, asset)
}

func TestGetOptionContracts(t *testing.T) {
	c := DefaultClient
	// successful case with simple query
	request := GetOptionContractsRequest{
		UnderlyingSymbols: "some_symbol1,some_symbol2",
	}
	expectedID := "some_id"
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, request.UnderlyingSymbols, req.URL.Query().Get("underlying_symbols"))
		response := optionContractsResponse{
			OptionContracts: []OptionContract{
				{
					ID:             expectedID,
					ExpirationDate: civil.Date{Year: 1, Month: 1, Day: 1},
				},
			},
		}
		return &http.Response{
			Body: genBody(response),
		}, nil
	}

	contracts, err := c.GetOptionContracts(request)
	require.NoError(t, err)
	require.Len(t, contracts, 1)
	assert.Equal(t, expectedID, contracts[0].ID)

	// successful case with more paramters
	request = GetOptionContractsRequest{
		UnderlyingSymbols:     "some_symbol",
		ShowDeliverable:       true,
		Status:                OptionStatusActive,
		ExpirationDate:        civil.Date{Year: 2000, Month: 1, Day: 1},
		RootSymbol:            "some_symbol",
		Type:                  OptionTypeCall,
		Style:                 OptionStyleEuropean,
		StrikePriceGTE:        decimal.NewFromInt(10),
		TotalLimit:            1,
		PennyProgramIndicator: true,
	}
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, request.UnderlyingSymbols, req.URL.Query().Get("underlying_symbols"))
		assert.Equal(t, strconv.FormatBool(request.ShowDeliverable), req.URL.Query().Get("show_deliverables"))
		assert.Equal(t, string(request.Status), req.URL.Query().Get("status"))
		assert.Equal(t, request.ExpirationDate.String(), req.URL.Query().Get("expiration_date"))
		assert.Equal(t, request.RootSymbol, req.URL.Query().Get("root_symbol"))
		assert.Equal(t, string(request.Type), req.URL.Query().Get("type"))
		assert.Equal(t, string(request.Style), req.URL.Query().Get("style"))
		assert.Equal(t, request.StrikePriceGTE.String(), req.URL.Query().Get("strike_price_gte"))
		assert.Equal(t, strconv.Itoa(request.TotalLimit), req.URL.Query().Get("limit"))
		assert.Equal(t, strconv.FormatBool(request.PennyProgramIndicator), req.URL.Query().Get("ppind"))

		assets := optionContractsResponse{
			OptionContracts: []OptionContract{
				{
					ID:             expectedID,
					ExpirationDate: civil.Date{Year: 1, Month: 1, Day: 1},
				},
			},
		}
		return &http.Response{
			Body: genBody(assets),
		}, nil
	}

	contracts, err = c.GetOptionContracts(request)
	require.NoError(t, err)
	require.Len(t, contracts, 1)
	assert.Equal(t, expectedID, contracts[0].ID)

	// api failure
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		return &http.Response{}, errors.New("fail")
	}

	_, err = c.GetOptionContracts(GetOptionContractsRequest{})
	require.Error(t, err)
}

func TestGetOptionContract(t *testing.T) {
	c := DefaultClient
	// successful case
	expectedSymbol := "some_symbol"
	expectedID := "some_id"
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.True(t, strings.HasSuffix(req.URL.Path, expectedSymbol))

		return &http.Response{
			Body: genBody(OptionContract{
				ID:             expectedID,
				ExpirationDate: civil.Date{Year: 1, Month: 1, Day: 1},
			}),
		}, nil
	}

	contract, err := c.GetOptionContract(expectedSymbol)
	require.NoError(t, err)
	require.NotNil(t, contract)
	assert.Equal(t, expectedID, contract.ID)

	// api failure
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		return &http.Response{}, errors.New("fail")
	}

	_, err = c.GetOptionContract(expectedSymbol)
	require.Error(t, err)
}

func TestTestVerify(t *testing.T) {
	// 200
	resp := &http.Response{
		StatusCode: http.StatusOK,
	}

	require.NoError(t, verify(resp))

	// 500
	resp = &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       genBody(APIError{Code: 1010101, Message: "server is dead"}),
	}

	assert.Error(t, verify(resp))
}

func TestOTOCOOrders(t *testing.T) {
	c := DefaultClient
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		or := PlaceOrderRequest{}
		if err := json.NewDecoder(req.Body).Decode(&or); err != nil {
			return nil, err
		}
		return &http.Response{
			Body: genBody(Order{
				Qty:         or.Qty,
				Side:        or.Side,
				TimeInForce: or.TimeInForce,
				Type:        or.Type,
				OrderClass:  or.OrderClass,
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
		Qty:         &one,
		Side:        Buy,
		TimeInForce: GTC,
		Type:        Limit,
		OrderClass:  Bracket,
		TakeProfit:  tp,
		StopLoss:    sl,
	}

	order, err := c.PlaceOrder(req)
	require.NoError(t, err)
	assert.NotNil(t, order)
	assert.Equal(t, Bracket, order.OrderClass)
}

func TestGetAccountActivities(t *testing.T) {
	c := DefaultClient
	// happy path
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		// https://alpaca.markets/docs/api-documentation/api-v2/account-activities/#nontradeactivity-entity
		nta := []map[string]interface{}{
			{
				"activity_type":    "DIV",
				"id":               "20190801011955195::5f596936-6f23-4cef-bdf1-3806aae57dbf",
				"date":             "2019-08-01",
				"net_amount":       "1.02",
				"symbol":           "T",
				"qty":              "2",
				"per_share_amount": "0.51",
				"status":           "executed",
			},
			{
				"activity_type":    "DIV",
				"id":               "20190801011955195::5f596936-6f23-4cef-bdf1-3806aae57dbd",
				"date":             "2019-08-01",
				"net_amount":       "5",
				"symbol":           "AAPL",
				"qty":              "2",
				"per_share_amount": "100",
				"status":           "executed",
			},
			{
				"activity_type":    "FILL",
				"id":               "20240624093004214::18a82342-245e-4e8a-9703-87ae38d9b629",
				"transaction_time": "2024-06-24T13:30:04.214535Z",
				"type":             "partial_fill",
				"price":            "3.8",
				"qty":              "643",
				"side":             "sell",
				"symbol":           "AAPL",
				"leaves_qty":       "1457",
				"order_id":         "c0e497c2-a547-48cd-85dc-0f1f0ed1b26c",
				"cum_qty":          "643",
				"order_status":     "partially_filled",
			},
		}
		return &http.Response{
			Body: genBody(nta),
		}, nil
	}

	activities, err := c.GetAccountActivities(GetAccountActivitiesRequest{
		ActivityTypes: []string{"DIV", "FILL"},
	})
	require.NoError(t, err)
	assert.Len(t, activities, 3)
	activity1 := activities[0]
	assert.Equal(t, civil.Date{Year: 2019, Month: 8, Day: 1}, activity1.Date)
	assert.Equal(t, "DIV", activity1.ActivityType)
	assert.Equal(t, "20190801011955195::5f596936-6f23-4cef-bdf1-3806aae57dbf", activity1.ID)
	assert.True(t, decimal.NewFromFloat(1.02).Equal(activity1.NetAmount))
	assert.Equal(t, "T", activity1.Symbol)
	assert.Equal(t, decimal.NewFromInt(2), activity1.Qty)
	assert.Equal(t, decimal.NewFromFloat32(0.51), activity1.PerShareAmount)
	assert.Equal(t, "executed", activity1.Status)
	activity2 := activities[1]
	assert.Equal(t, civil.Date{Year: 2019, Month: 8, Day: 1}, activity2.Date)
	assert.Equal(t, "DIV", activity2.ActivityType)
	assert.Equal(t, "20190801011955195::5f596936-6f23-4cef-bdf1-3806aae57dbd", activity2.ID)
	assert.True(t, decimal.NewFromInt(5).Equal(activity2.NetAmount))
	assert.Equal(t, "AAPL", activity2.Symbol)
	assert.Equal(t, decimal.NewFromInt(2), activity2.Qty)
	assert.Equal(t, decimal.NewFromInt(100), activity2.PerShareAmount)
	assert.Equal(t, "executed", activity2.Status)
	activity3 := activities[2]
	assert.Equal(t, "FILL", activity3.ActivityType)
	assert.Equal(t, "20240624093004214::18a82342-245e-4e8a-9703-87ae38d9b629", activity3.ID)
	assert.Equal(t, "2024-06-24T13:30:04.214535Z", activity3.TransactionTime.Format("2006-01-02T15:04:05.999999Z"))
	assert.Equal(t, "partial_fill", activity3.Type)
	assert.True(t, decimal.NewFromFloat32(3.8).Equal(activity3.Price))
	assert.Equal(t, decimal.NewFromInt(643), activity3.Qty)
	assert.Equal(t, "sell", activity3.Side)
	assert.Equal(t, "AAPL", activity3.Symbol)
	assert.Equal(t, decimal.NewFromInt(1457), activity3.LeavesQty)
	assert.Equal(t, "c0e497c2-a547-48cd-85dc-0f1f0ed1b26c", activity3.OrderID)
	assert.Equal(t, decimal.NewFromInt(643), activity3.CumQty)
	assert.Equal(t, "partially_filled", activity3.OrderStatus)

	// error was returned
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		return &http.Response{}, &APIError{StatusCode: 500, Message: "internal server error"}
	}

	_, err = c.GetAccountActivities(GetAccountActivitiesRequest{})
	require.Error(t, err)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, 500, apiErr.StatusCode)
	assert.Equal(t, "internal server error", apiErr.Message)

	// test filter by date and URI
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		getQuery := req.URL.Query()

		assert.Equal(t, "/v2/account/activities", req.URL.Path)
		assert.Equal(t, "DIV", getQuery.Get("activity_types"))
		assert.Equal(t, "2019-01-01T00:00:00.0000001Z", getQuery.Get("after"))
		assert.Equal(t, "10", getQuery.Get("page_size"))

		nta := []map[string]interface{}{
			{
				"activity_type":    "DIV",
				"id":               "20190801011955195::5f596936-6f23-4cef-bdf1-3806aae57dbf",
				"date":             "2019-08-01",
				"net_amount":       "1.02",
				"symbol":           "T",
				"qty":              "2",
				"per_share_amount": "0.51",
				"status":           "executed",
			},
		}
		return &http.Response{
			Body: genBody(nta),
		}, nil
	}

	_, err = c.GetAccountActivities(GetAccountActivitiesRequest{
		ActivityTypes: []string{"DIV"},
		After:         time.Date(2019, 1, 1, 0, 0, 0, 100, time.UTC),
		PageSize:      10,
	})
	assert.NoError(t, err)
}

func TestGetUSTreasuries(t *testing.T) {
	c := DefaultClient

	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v2/assets/fixed_income/us_treasuries", req.URL.Path)
		assert.Equal(t, http.MethodGet, req.Method)
		assert.Equal(t, "bill", req.URL.Query().Get("subtype"))
		assert.Equal(t, "outstanding", req.URL.Query().Get("bond_status"))
		assert.Equal(t, "US912797KJ59,US912797KJ60", req.URL.Query().Get("isins"))
		resp := `{"us_treasuries":[
			{
				"cusip": "912797KJ5",
				"isin": "US912797KJ59",
				"bond_status": "outstanding",
				"tradable": true,
				"subtype": "bill",
				"issue_date": "2026-04-03",
				"maturity_date": "2026-07-03",
				"description": "test bill",
				"description_short": "tb",
				"close_price": 99.6459,
				"close_price_date": "2026-04-29",
				"close_yield_to_maturity": 4.249,
				"close_yield_to_worst": 4.249,
				"coupon": 0,
				"coupon_type": "zero",
				"coupon_frequency": "zero"
			},
			{
				"cusip": "912797KJ6",
				"isin": "US912797KJ60",
				"bond_status": "outstanding",
				"tradable": true,
				"subtype": "bill",
				"issue_date": "2026-05-01",
				"maturity_date": "2026-08-01",
				"description": "test bill 2",
				"description_short": "tb2",
				"coupon": 0,
				"coupon_type": "zero",
				"coupon_frequency": "zero",
				"first_coupon_date": "2026-11-01",
				"next_coupon_date": "2026-11-01",
				"last_coupon_date": "2026-05-01"
			}
		]}`
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(resp)),
		}, nil
	}

	got, err := c.GetUSTreasuries(GetUSTreasuriesRequest{
		Subtype:    TreasurySubtypeBill,
		BondStatus: BondStatusOutstanding,
		ISINs:      []string{"US912797KJ59", "US912797KJ60"},
	})
	require.NoError(t, err)
	require.Len(t, got, 2)

	assert.Equal(t, "912797KJ5", got[0].CUSIP)
	assert.Equal(t, "US912797KJ59", got[0].ISIN)
	assert.Equal(t, BondStatusOutstanding, got[0].BondStatus)
	assert.True(t, got[0].Tradable)
	assert.Equal(t, TreasurySubtypeBill, got[0].Subtype)
	assert.Equal(t, civil.Date{Year: 2026, Month: 4, Day: 3}, got[0].IssueDate)
	assert.Equal(t, civil.Date{Year: 2026, Month: 7, Day: 3}, got[0].MaturityDate)
	assert.Equal(t, "test bill", got[0].Description)
	assert.Equal(t, "tb", got[0].DescriptionShort)
	require.NotNil(t, got[0].ClosePrice)
	assert.True(t, decimal.NewFromFloat(99.6459).Equal(*got[0].ClosePrice))
	require.NotNil(t, got[0].ClosePriceDate)
	assert.Equal(t, civil.Date{Year: 2026, Month: 4, Day: 29}, *got[0].ClosePriceDate)
	require.NotNil(t, got[0].CloseYieldToMaturity)
	assert.True(t, decimal.NewFromFloat(4.249).Equal(*got[0].CloseYieldToMaturity))
	require.NotNil(t, got[0].CloseYieldToWorst)
	assert.True(t, decimal.NewFromFloat(4.249).Equal(*got[0].CloseYieldToWorst))
	assert.True(t, decimal.Zero.Equal(got[0].Coupon))
	assert.Equal(t, CouponTypeZero, got[0].CouponType)
	assert.Equal(t, CouponFrequencyZero, got[0].CouponFrequency)
	assert.Nil(t, got[0].FirstCouponDate)
	assert.Nil(t, got[0].NextCouponDate)
	assert.Nil(t, got[0].LastCouponDate)

	assert.Equal(t, "912797KJ6", got[1].CUSIP)
	assert.Nil(t, got[1].ClosePrice)
	assert.Nil(t, got[1].ClosePriceDate)
	require.NotNil(t, got[1].FirstCouponDate)
	assert.Equal(t, civil.Date{Year: 2026, Month: 11, Day: 1}, *got[1].FirstCouponDate)
	require.NotNil(t, got[1].NextCouponDate)
	assert.Equal(t, civil.Date{Year: 2026, Month: 11, Day: 1}, *got[1].NextCouponDate)
	require.NotNil(t, got[1].LastCouponDate)
	assert.Equal(t, civil.Date{Year: 2026, Month: 5, Day: 1}, *got[1].LastCouponDate)

	// empty request sends no query params
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "", req.URL.Query().Get("subtype"))
		assert.Equal(t, "", req.URL.Query().Get("bond_status"))
		assert.Equal(t, "", req.URL.Query().Get("cusips"))
		assert.Equal(t, "", req.URL.Query().Get("isins"))
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(`{"us_treasuries":[]}`)),
		}, nil
	}
	got, err = c.GetUSTreasuries(GetUSTreasuriesRequest{})
	require.NoError(t, err)
	assert.Empty(t, got)

	// api failure
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		return &http.Response{}, errors.New("fail")
	}
	_, err = c.GetUSTreasuries(GetUSTreasuriesRequest{})
	require.Error(t, err)
}

func TestGetUSCorporates(t *testing.T) {
	c := DefaultClient

	spOutlook := SPOutlookStable
	callType := CallTypeMakeWhole
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v2/assets/fixed_income/us_corporates", req.URL.Path)
		assert.Equal(t, http.MethodGet, req.Method)
		assert.Equal(t, "outstanding", req.URL.Query().Get("bond_status"))
		assert.Equal(t, "BAC,MSFT", req.URL.Query().Get("tickers"))
		resp := `{"us_corporates":[
			{
				"cusip": "06051GJH9",
				"isin": "US06051GJH92",
				"bond_status": "outstanding",
				"tradable": true,
				"marginable": true,
				"issue_date": "2025-06-15",
				"maturity_date": "2030-06-15",
				"country_domicile": "US",
				"ticker": "BAC",
				"seniority": "senior_unsecured",
				"issuer": "Bank of America Corporation",
				"sector": "Financials",
				"description": "test bond",
				"description_short": "tb",
				"coupon": 3.5,
				"coupon_type": "fixed",
				"coupon_frequency": "semi_annual",
				"first_coupon_date": "2025-12-15",
				"next_coupon_date": "2026-06-15",
				"last_coupon_date": "2025-12-15",
				"perpetual": false,
				"day_count": "30/360",
				"dated_date": "2025-06-15",
				"issue_size": 2000000000,
				"issue_price": 99.876,
				"issue_minimum_denomination": 1000,
				"par_value": 1000,
				"callable": true,
				"next_call_date": "2026-05-15",
				"next_call_price": 100,
				"puttable": false,
				"convertible": false,
				"reg_s": false,
				"sp_rating": "A-",
				"sp_rating_date": "2026-01-10",
				"sp_creditwatch": "positive",
				"sp_creditwatch_date": "2026-01-10",
				"sp_outlook": "stable",
				"sp_outlook_date": "2026-01-10",
				"liquidity_micro_buy": 4,
				"liquidity_micro_sell": 3,
				"liquidity_micro_aggregate": 3.5,
				"liquidity_retail_buy": 5,
				"liquidity_retail_sell": 4,
				"liquidity_retail_aggregate": 4.5,
				"liquidity_institutional_buy": 5,
				"liquidity_institutional_sell": 5,
				"liquidity_institutional_aggregate": 5,
				"close_price": 99.95,
				"close_price_date": "2026-05-02",
				"close_yield_to_maturity": 3.55,
				"close_yield_to_worst": 3.45,
				"accrued_interest": 4.375,
				"call_type": "make_whole",
				"reissue_date": "2026-01-15",
				"reissue_size": 500000000,
				"reissue_price": 101.5
			}
		]}`
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(resp)),
		}, nil
	}

	got, err := c.GetUSCorporates(GetUSCorporatesRequest{
		BondStatus: BondStatusOutstanding,
		Tickers:    []string{"BAC", "MSFT"},
	})
	require.NoError(t, err)
	require.Len(t, got, 1)

	bond := got[0]
	assert.Equal(t, "06051GJH9", bond.CUSIP)
	assert.Equal(t, "US06051GJH92", bond.ISIN)
	assert.Equal(t, BondStatusOutstanding, bond.BondStatus)
	assert.True(t, bond.Tradable)
	assert.True(t, bond.Marginable)
	assert.Equal(t, civil.Date{Year: 2025, Month: 6, Day: 15}, bond.IssueDate)
	require.NotNil(t, bond.MaturityDate)
	assert.Equal(t, civil.Date{Year: 2030, Month: 6, Day: 15}, *bond.MaturityDate)
	assert.Equal(t, "US", bond.CountryDomicile)
	assert.Equal(t, "BAC", bond.Ticker)
	assert.Equal(t, "senior_unsecured", bond.Seniority)
	assert.Equal(t, "Bank of America Corporation", bond.Issuer)
	assert.Equal(t, "Financials", bond.Sector)
	assert.Equal(t, "test bond", bond.Description)
	assert.Equal(t, "tb", bond.DescriptionShort)
	assert.True(t, decimal.NewFromFloat(3.5).Equal(bond.Coupon))
	assert.Equal(t, CouponTypeFixed, bond.CouponType)
	assert.Equal(t, CouponFrequencySemiAnnual, bond.CouponFrequency)
	require.NotNil(t, bond.FirstCouponDate)
	assert.Equal(t, civil.Date{Year: 2025, Month: 12, Day: 15}, *bond.FirstCouponDate)
	require.NotNil(t, bond.NextCouponDate)
	assert.Equal(t, civil.Date{Year: 2026, Month: 6, Day: 15}, *bond.NextCouponDate)
	require.NotNil(t, bond.LastCouponDate)
	assert.Equal(t, civil.Date{Year: 2025, Month: 12, Day: 15}, *bond.LastCouponDate)
	assert.False(t, bond.Perpetual)
	assert.Equal(t, DayCount30360, bond.DayCount)
	assert.Equal(t, civil.Date{Year: 2025, Month: 6, Day: 15}, bond.DatedDate)
	assert.True(t, decimal.NewFromFloat(2000000000).Equal(bond.IssueSize))
	assert.True(t, decimal.NewFromFloat(99.876).Equal(bond.IssuePrice))
	assert.True(t, decimal.NewFromFloat(1000).Equal(bond.IssueMinimumDenomination))
	assert.True(t, decimal.NewFromFloat(1000).Equal(bond.ParValue))
	assert.True(t, bond.Callable)
	require.NotNil(t, bond.NextCallDate)
	assert.Equal(t, civil.Date{Year: 2026, Month: 5, Day: 15}, *bond.NextCallDate)
	require.NotNil(t, bond.NextCallPrice)
	assert.True(t, decimal.NewFromFloat(100).Equal(*bond.NextCallPrice))
	assert.False(t, bond.Puttable)
	assert.False(t, bond.Convertible)
	assert.False(t, bond.RegS)
	require.NotNil(t, bond.SPRating)
	assert.Equal(t, "A-", *bond.SPRating)
	require.NotNil(t, bond.SPRatingDate)
	assert.Equal(t, civil.Date{Year: 2026, Month: 1, Day: 10}, *bond.SPRatingDate)
	require.NotNil(t, bond.SPCreditwatch)
	assert.Equal(t, "positive", *bond.SPCreditwatch)
	require.NotNil(t, bond.SPCreditwatchDate)
	assert.Equal(t, civil.Date{Year: 2026, Month: 1, Day: 10}, *bond.SPCreditwatchDate)
	require.NotNil(t, bond.SPOutlook)
	assert.Equal(t, &spOutlook, bond.SPOutlook)
	require.NotNil(t, bond.SPOutlookDate)
	assert.Equal(t, civil.Date{Year: 2026, Month: 1, Day: 10}, *bond.SPOutlookDate)
	require.NotNil(t, bond.LiquidityMicroBuy)
	assert.True(t, decimal.NewFromFloat(4).Equal(*bond.LiquidityMicroBuy))
	require.NotNil(t, bond.LiquidityMicroSell)
	assert.True(t, decimal.NewFromFloat(3).Equal(*bond.LiquidityMicroSell))
	require.NotNil(t, bond.LiquidityMicroAggregate)
	assert.True(t, decimal.NewFromFloat(3.5).Equal(*bond.LiquidityMicroAggregate))
	require.NotNil(t, bond.LiquidityRetailBuy)
	assert.True(t, decimal.NewFromFloat(5).Equal(*bond.LiquidityRetailBuy))
	require.NotNil(t, bond.LiquidityRetailSell)
	assert.True(t, decimal.NewFromFloat(4).Equal(*bond.LiquidityRetailSell))
	require.NotNil(t, bond.LiquidityRetailAggregate)
	assert.True(t, decimal.NewFromFloat(4.5).Equal(*bond.LiquidityRetailAggregate))
	require.NotNil(t, bond.LiquidityInstitutionalBuy)
	assert.True(t, decimal.NewFromFloat(5).Equal(*bond.LiquidityInstitutionalBuy))
	require.NotNil(t, bond.LiquidityInstitutionalSell)
	assert.True(t, decimal.NewFromFloat(5).Equal(*bond.LiquidityInstitutionalSell))
	require.NotNil(t, bond.LiquidityInstitutionalAggregate)
	assert.True(t, decimal.NewFromFloat(5).Equal(*bond.LiquidityInstitutionalAggregate))
	require.NotNil(t, bond.ClosePrice)
	assert.True(t, decimal.NewFromFloat(99.95).Equal(*bond.ClosePrice))
	require.NotNil(t, bond.ClosePriceDate)
	assert.Equal(t, civil.Date{Year: 2026, Month: 5, Day: 2}, *bond.ClosePriceDate)
	require.NotNil(t, bond.CloseYieldToMaturity)
	assert.True(t, decimal.NewFromFloat(3.55).Equal(*bond.CloseYieldToMaturity))
	require.NotNil(t, bond.CloseYieldToWorst)
	assert.True(t, decimal.NewFromFloat(3.45).Equal(*bond.CloseYieldToWorst))
	require.NotNil(t, bond.AccruedInterest)
	assert.True(t, decimal.NewFromFloat(4.375).Equal(*bond.AccruedInterest))
	require.NotNil(t, bond.CallType)
	assert.Equal(t, &callType, bond.CallType)
	require.NotNil(t, bond.ReissueDate)
	assert.Equal(t, civil.Date{Year: 2026, Month: 1, Day: 15}, *bond.ReissueDate)
	require.NotNil(t, bond.ReissueSize)
	assert.True(t, decimal.NewFromFloat(500000000).Equal(*bond.ReissueSize))
	require.NotNil(t, bond.ReissuePrice)
	assert.True(t, decimal.NewFromFloat(101.5).Equal(*bond.ReissuePrice))

	// filtering by cusips and isins
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "06051GJH9", req.URL.Query().Get("cusips"))
		assert.Equal(t, "US06051GJH92", req.URL.Query().Get("isins"))
		assert.Equal(t, "", req.URL.Query().Get("tickers"))
		assert.Equal(t, "", req.URL.Query().Get("bond_status"))
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(`{"us_corporates":[]}`)),
		}, nil
	}
	got, err = c.GetUSCorporates(GetUSCorporatesRequest{
		CUSIPs: []string{"06051GJH9"},
		ISINs:  []string{"US06051GJH92"},
	})
	require.NoError(t, err)
	assert.Empty(t, got)

	// api failure
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		return &http.Response{}, errors.New("fail")
	}
	_, err = c.GetUSCorporates(GetUSCorporatesRequest{})
	require.Error(t, err)
}

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

func genBody(data interface{}) io.ReadCloser {
	buf, _ := json.Marshal(data)
	return nopCloser{bytes.NewBuffer(buf)}
}
