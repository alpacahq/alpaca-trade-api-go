package alpaca

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAccount(t *testing.T) {
	c := DefaultClient

	// successful
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
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
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
		return &http.Response{}, fmt.Errorf("fail")
	}

	_, err = c.GetAccount()
	require.Error(t, err)
}

func TestGetPositions(t *testing.T) {
	c := DefaultClient

	// successful
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
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
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
		return &http.Response{}, fmt.Errorf("fail")
	}

	positions, err = c.GetPositions()
	require.Error(t, err)
	assert.Nil(t, positions)
}

func TestGetClock(t *testing.T) {
	c := DefaultClient
	// successful
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
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
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
		return &http.Response{}, fmt.Errorf("fail")
	}

	_, err = c.GetClock()
	require.Error(t, err)
}

func TestGetCalendar(t *testing.T) {
	c := DefaultClient
	// successful
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
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
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
		return &http.Response{}, fmt.Errorf("fail")
	}

	calendar, err = c.GetCalendar(GetCalendarRequest{})
	require.Error(t, err)
	assert.Nil(t, calendar)
}

func TestGetOrders_EmptyRequest(t *testing.T) {
	c := DefaultClient
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
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
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
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
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
		return &http.Response{}, fmt.Errorf("fail")
	}

	orders, err = c.GetOrders(req)
	require.Error(t, err)
	assert.Nil(t, orders)
}

func TestPlaceOrder(t *testing.T) {
	c := DefaultClient
	// successful (w/ Qty)
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
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

	// api failure
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
		return &http.Response{}, fmt.Errorf("fail")
	}

	_, err = c.PlaceOrder(req)
	require.Error(t, err)
}

func TestGetOrder(t *testing.T) {
	c := DefaultClient
	// successful
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
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
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
		return &http.Response{}, fmt.Errorf("fail")
	}

	_, err = c.GetOrder("some_order_id")
	require.Error(t, err)
}

func TestGetOrderByClientOrderId(t *testing.T) {
	c := DefaultClient
	// successful
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
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
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
		return &http.Response{}, fmt.Errorf("fail")
	}

	_, err = c.GetOrderByClientOrderID("some_client_order_id")
	require.Error(t, err)
}

func TestClient_GetAnnouncements(t *testing.T) {
	c := DefaultClient
	// successful
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v2/corporate_actions/announcements", req.URL.Path)
		assert.Equal(t, "GET", req.Method)
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
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v2/corporate_actions/announcements/123", req.URL.Path)
		assert.Equal(t, "GET", req.Method)

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

func TestCancelOrder(t *testing.T) {
	c := DefaultClient
	// successful
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
		return &http.Response{}, nil
	}

	assert.Nil(t, c.CancelOrder("some_order_id"))

	// api failure
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
		return &http.Response{}, fmt.Errorf("fail")
	}

	assert.NotNil(t, c.CancelOrder("some_order_id"))
}

func TestGetAssets(t *testing.T) {
	c := DefaultClient
	// successful
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
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
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
		return &http.Response{}, fmt.Errorf("fail")
	}

	_, err = c.GetAssets(GetAssetsRequest{})
	require.Error(t, err)
}

func TestGetAsset(t *testing.T) {
	c := DefaultClient
	// successful
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
		asset := Asset{ID: "some_id"}
		return &http.Response{
			Body: genBody(asset),
		}, nil
	}

	asset, err := c.GetAsset("APCA")
	require.NoError(t, err)
	assert.NotNil(t, asset)

	// api failure
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
		return &http.Response{}, fmt.Errorf("fail")
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
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(assetJSON)),
		}, nil
	}

	asset, err := c.GetAsset("APCA")
	assert.Nil(t, err)
	assert.Equal(t, "us_equity", asset.Class)
	assert.True(t, asset.Fractionable)
	assert.NotNil(t, asset)

	// api failure
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
		return &http.Response{}, fmt.Errorf("fail")
	}

	asset, err = c.GetAsset("APCA")
	assert.NotNil(t, err)
	assert.Nil(t, asset)
}

func TestTestVerify(t *testing.T) {
	// 200
	resp := &http.Response{
		StatusCode: http.StatusOK,
	}

	assert.Nil(t, verify(resp))

	// 500
	resp = &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       genBody(APIError{Code: 1010101, Message: "server is dead"}),
	}

	assert.NotNil(t, verify(resp))
}

func TestOTOCOOrders(t *testing.T) {
	c := DefaultClient
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
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
	assert.Equal(t, "bracket", order.Class)
}

func TestGetAccountActivities(t *testing.T) {
	c := DefaultClient
	// happy path
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
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
			},
			{
				"activity_type":    "DIV",
				"id":               "20190801011955195::5f596936-6f23-4cef-bdf1-3806aae57dbd",
				"date":             "2019-08-01",
				"net_amount":       "5",
				"symbol":           "AAPL",
				"qty":              "2",
				"per_share_amount": "100",
			},
		}
		return &http.Response{
			Body: genBody(nta),
		}, nil
	}

	activities, err := c.GetAccountActivities(GetAccountActivitiesRequest{
		ActivityTypes: []string{"DIV"},
	})
	assert.NoError(t, err)
	assert.Len(t, activities, 2)
	activity1 := activities[0]
	assert.Equal(t, civil.Date{Year: 2019, Month: 8, Day: 1}, activity1.Date)
	assert.Equal(t, "DIV", activity1.ActivityType)
	assert.Equal(t, "20190801011955195::5f596936-6f23-4cef-bdf1-3806aae57dbf", activity1.ID)
	assert.True(t, decimal.NewFromFloat(1.02).Equal(activity1.NetAmount))
	assert.Equal(t, "T", activity1.Symbol)
	assert.Equal(t, decimal.NewFromInt(2), activity1.Qty)
	assert.Equal(t, decimal.NewFromFloat32(0.51), activity1.PerShareAmount)
	activity2 := activities[1]
	assert.Equal(t, civil.Date{Year: 2019, Month: 8, Day: 1}, activity2.Date)
	assert.Equal(t, "DIV", activity2.ActivityType)
	assert.Equal(t, "20190801011955195::5f596936-6f23-4cef-bdf1-3806aae57dbd", activity2.ID)
	assert.True(t, decimal.NewFromInt(5).Equal(activity2.NetAmount))
	assert.Equal(t, "AAPL", activity2.Symbol)
	assert.Equal(t, decimal.NewFromInt(2), activity2.Qty)
	assert.Equal(t, decimal.NewFromInt(100), activity2.PerShareAmount)

	// error was returned
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
		return &http.Response{}, &APIError{Code: 500, Message: "internal server error"}
	}

	_, err = c.GetAccountActivities(GetAccountActivitiesRequest{})
	assert.NotNil(t, err)
	assert.EqualError(t, &APIError{Code: 500, Message: "internal server error"}, "internal server error")

	// test filter by date and URI
	c.do = func(c *Client, req *http.Request) (*http.Response, error) {
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

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

func genBody(data interface{}) io.ReadCloser {
	buf, _ := json.Marshal(data)
	return nopCloser{bytes.NewBuffer(buf)}
}
