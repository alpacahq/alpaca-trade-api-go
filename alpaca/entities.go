package alpaca

import (
	"time"

	"github.com/shopspring/decimal"
)

type Account struct {
	ID                    string          `json:"id"`
	AccountNumber         string          `json:"account_number"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
	DeletedAt             *time.Time      `json:"deleted_at"`
	Status                string          `json:"status"`
	Currency              string          `json:"currency"`
	Cash                  decimal.Decimal `json:"cash"`
	CashWithdrawable      decimal.Decimal `json:"cash_withdrawable"`
	TradingBlocked        bool            `json:"trading_blocked"`
	TransfersBlocked      bool            `json:"transfers_blocked"`
	AccountBlocked        bool            `json:"account_blocked"`
	ShortingEnabled       bool            `json:"shorting_enabled"`
	BuyingPower           decimal.Decimal `json:"buying_power"`
	PatternDayTrader      bool            `json:"pattern_day_trader"`
	DaytradeCount         int64           `json:"daytrade_count"`
	DaytradingBuyingPower decimal.Decimal `json:"daytrading_buying_power"`
	RegTBuyingPower       decimal.Decimal `json:"regt_buying_power"`
	Equity                decimal.Decimal `json:"equity"`
	LastEquity            decimal.Decimal `json:"last_equity"`
	Multiplier            string          `json:"multiplier"`
	InitialMargin         decimal.Decimal `json:"initial_margin"`
	MaintenanceMargin     decimal.Decimal `json:"maintenance_margin"`
	LastMaintenanceMargin decimal.Decimal `json:"last_maintenance_margin"`
	LongMarketValue       decimal.Decimal `json:"long_market_value"`
	ShortMarketValue      decimal.Decimal `json:"short_market_value"`
	PortfolioValue        decimal.Decimal `json:"portfolio_value"`
}

type Order struct {
	ID             string           `json:"id"`
	ClientOrderID  string           `json:"client_order_id"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	SubmittedAt    time.Time        `json:"submitted_at"`
	FilledAt       *time.Time       `json:"filled_at"`
	ExpiredAt      *time.Time       `json:"expired_at"`
	CanceledAt     *time.Time       `json:"canceled_at"`
	FailedAt       *time.Time       `json:"failed_at"`
	ReplacedAt     *time.Time       `json:"replaced_at"`
	Replaces       *string          `json:"replaces"`
	AssetID        string           `json:"asset_id"`
	Symbol         string           `json:"symbol"`
	Exchange       string           `json:"exchange"`
	Class          string           `json:"asset_class"`
	Qty            decimal.Decimal  `json:"qty"`
	FilledQty      decimal.Decimal  `json:"filled_qty"`
	Type           OrderType        `json:"order_type"`
	Side           Side             `json:"side"`
	TimeInForce    TimeInForce      `json:"time_in_force"`
	LimitPrice     *decimal.Decimal `json:"limit_price"`
	FilledAvgPrice *decimal.Decimal `json:"filled_avg_price"`
	StopPrice      *decimal.Decimal `json:"stop_price"`
	TrailPrice     *decimal.Decimal `json:"trail_price"`
	TrailPercent   *decimal.Decimal `json:"trail_percent"`
	Hwm            *decimal.Decimal `json:"hwm"`
	Status         string           `json:"status"`
	ExtendedHours  bool             `json:"extended_hours"`
	Legs           *[]Order         `json:"legs"`
}

type Position struct {
	AssetID        string          `json:"asset_id"`
	Symbol         string          `json:"symbol"`
	Exchange       string          `json:"exchange"`
	Class          string          `json:"asset_class"`
	AccountID      string          `json:"account_id"`
	EntryPrice     decimal.Decimal `json:"avg_entry_price"`
	Qty            decimal.Decimal `json:"qty"`
	Side           string          `json:"side"`
	MarketValue    decimal.Decimal `json:"market_value"`
	CostBasis      decimal.Decimal `json:"cost_basis"`
	UnrealizedPL   decimal.Decimal `json:"unrealized_pl"`
	UnrealizedPLPC decimal.Decimal `json:"unrealized_plpc"`
	CurrentPrice   decimal.Decimal `json:"current_price"`
	LastdayPrice   decimal.Decimal `json:"lastday_price"`
	ChangeToday    decimal.Decimal `json:"change_today"`
}

type Asset struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Exchange     string `json:"exchange"`
	Class        string `json:"asset_class"`
	Symbol       string `json:"symbol"`
	Status       string `json:"status"`
	Tradable     bool   `json:"tradable"`
	Marginable   bool   `json:"marginable"`
	Shortable    bool   `json:"shortable"`
	EasyToBorrow bool   `json:"easy_to_borrow"`
}

type Fundamental struct {
	AssetID           string          `json:"asset_id"`
	Symbol            string          `json:"symbol"`
	FullName          string          `json:"full_name"`
	IndustryName      string          `json:"industry_name"`
	IndustryGroup     string          `json:"industry_group"`
	Sector            string          `json:"sector"`
	PERatio           float32         `json:"pe_ratio"`
	PEGRatio          float32         `json:"peg_ratio"`
	Beta              float32         `json:"beta"`
	EPS               float32         `json:"eps"`
	MarketCap         int64           `json:"market_cap"`
	SharesOutstanding int64           `json:"shares_outstanding"`
	AvgVol            int64           `json:"avg_vol"`
	DivRate           float32         `json:"div_rate"`
	ROE               float32         `json:"roe"`
	ROA               float32         `json:"roa"`
	PS                float32         `json:"ps"`
	PC                float32         `json:"pc"`
	GrossMargin       float32         `json:"gross_margin"`
	FiftyTwoWeekHigh  decimal.Decimal `json:"fifty_two_week_high"`
	FiftyTwoWeekLow   decimal.Decimal `json:"fifty_two_week_low"`
	ShortDescription  string          `json:"short_description"`
	LongDescription   string          `json:"long_description"`
}

type Bar struct {
	Time   int64   `json:"t"`
	Open   float32 `json:"o"`
	High   float32 `json:"h"`
	Low    float32 `json:"l"`
	Close  float32 `json:"c"`
	Volume int32   `json:"v"`
}

type ListBarParams struct {
	Timeframe string     `url:"timeframe,omitempty"`
	StartDt   *time.Time `url:"start_dt,omitempty"`
	EndDt     *time.Time `url:"end_dt,omitempty"`
	Limit     *int       `url:"limit,omitempty"`
}

type LastQuote struct {
	AskPrice    float32 `json:"askprice"`
	AskSize     int32   `json:"asksize"`
	AskExchange int     `json:"askexchange"`
	BidPrice    float32 `json:"bidprice"`
	BidSize     int32   `json:"bidsize"`
	BidExchange int     `json:"bidexchange"`
	Timestamp   int64   `json:"timestamp"`
}

func (l *LastQuote) Time() time.Time {
	return time.Unix(0, l.Timestamp)
}

type LastQuoteResponse struct {
	Status string    `json:"status"`
	Symbol string    `json:"symbol"`
	Last   LastQuote `json:"last"`
}

type LastTrade struct {
	Price     float32 `json:"price"`
	Size      int32   `json:"size"`
	Exchange  int     `json:"exchange"`
	Cond1     int     `json:"cond1"`
	Cond2     int     `json:"cond2"`
	Cond3     int     `json:"cond3"`
	Cond4     int     `json:"cond4"`
	Timestamp int64   `json:"timestamp"`
}

func (l *LastTrade) Time() time.Time {
	return time.Unix(0, l.Timestamp)
}

type LastTradeResponse struct {
	Status string    `json:"status"`
	Symbol string    `json:"symbol"`
	Last   LastTrade `json:"last"`
}

type AggV2 struct {
	Timestamp     int64   `json:"t"`
	Ticker        string  `json:"T"`
	Open          float32 `json:"O"`
	High          float32 `json:"H"`
	Low           float32 `json:"L"`
	Close         float32 `json:"C"`
	Volume        int32   `json:"V"`
	NumberOfItems int     `json:"n"`
}

type Aggregates struct {
	Ticker       string  `json:"ticker"`
	Status       string  `json:"status"`
	Adjusted     bool    `json:"adjusted"`
	QueryCount   int     `json:"queryCount"`
	ResultsCount int     `json:"resultsCount"`
	Results      []AggV2 `json:"results"`
}

type CalendarDay struct {
	Date  string `json:"date"`
	Open  string `json:"open"`
	Close string `json:"close"`
}

type Clock struct {
	Timestamp time.Time `json:"timestamp"`
	IsOpen    bool      `json:"is_open"`
	NextOpen  time.Time `json:"next_open"`
	NextClose time.Time `json:"next_close"`
}

type AccountConfigurations struct {
	DtbpCheck            DtbpCheck         `json:"dtbp_check"`
	NoShorting           bool              `json:"no_shorting"`
	TradeConfirmEmail    TradeConfirmEmail `json:"trade_confirm_email"`
	TradeSuspendedByUser bool              `json:"trade_suspended_by_user"`
}

type AccountActivity struct {
	ID              string          `json:"id"`
	ActivityType    string          `json:"activity_type"`
	TransactionTime time.Time       `json:"transaction_time"`
	Type            string          `json:"type"`
	Price           decimal.Decimal `json:"price"`
	Qty             decimal.Decimal `json:"qty"`
	Side            string          `json:"side"`
	Symbol          string          `json:"symbol"`
	LeavesQty       decimal.Decimal `json:"leaves_qty"`
	CumQty          decimal.Decimal `json:"cum_qty"`
	Date            time.Time       `json:"date"`
	NetAmount       decimal.Decimal `json:"net_amount"`
	Description     string          `json:"description"`
	PerShareAmount  decimal.Decimal `json:"per_share_amount"`
}

type PortfolioHistory struct {
	BaseValue     decimal.Decimal   `json:"base_value"`
	Equity        []decimal.Decimal `json:"equity"`
	ProfitLoss    []decimal.Decimal `json:"profit_loss"`
	ProfitLossPct []decimal.Decimal `json:"profit_loss_pct"`
	Timeframe     RangeFreq         `json:"timeframe"`
	Timestamp     []int64           `json:"timestamp"`
}

type PlaceOrderRequest struct {
	AccountID     string           `json:"-"`
	AssetKey      *string          `json:"symbol"`
	Qty           decimal.Decimal  `json:"qty"`
	Side          Side             `json:"side"`
	Type          OrderType        `json:"type"`
	TimeInForce   TimeInForce      `json:"time_in_force"`
	LimitPrice    *decimal.Decimal `json:"limit_price"`
	StopPrice     *decimal.Decimal `json:"stop_price"`
	ClientOrderID string           `json:"client_order_id"`
	OrderClass    OrderClass       `json:"order_class"`
	TakeProfit    *TakeProfit      `json:"take_profit"`
	StopLoss      *StopLoss        `json:"stop_loss"`
	TrailPrice    *decimal.Decimal `json:"trail_price"`
	TrailPercent  *decimal.Decimal `json:"trail_percent"`
}

type TakeProfit struct {
	LimitPrice *decimal.Decimal `json:"limit_price"`
}

type StopLoss struct {
	LimitPrice *decimal.Decimal `json:"limit_price"`
	StopPrice  *decimal.Decimal `json:"stop_price"`
}

type OrderAttributes struct {
	TakeProfitLimitPrice *decimal.Decimal `json:"take_profit_limit_price,omitempty"`
	StopLossStopPrice    *decimal.Decimal `json:"stop_loss_stop_price,omitempty"`
	StopLossLimitPrice   *decimal.Decimal `json:"stop_loss_limit_price,omitempty"`
}

type ReplaceOrderRequest struct {
	Qty           *decimal.Decimal `json:"qty"`
	LimitPrice    *decimal.Decimal `json:"limit_price"`
	StopPrice     *decimal.Decimal `json:"stop_price"`
	Trail         *decimal.Decimal `json:"trail"`
	TimeInForce   TimeInForce      `json:"time_in_force"`
	ClientOrderID string           `json:"client_order_id"`
}

type AccountConfigurationsRequest struct {
	DtbpCheck            *string `json:"dtbp_check"`
	NoShorting           *bool   `json:"no_shorting"`
	TradeConfirmEmail    *string `json:"trade_confirm_email"`
	TradeSuspendedByUser *bool   `json:"trade_suspended_by_user"`
}

type AccountActivitiesRequest struct {
	ActivityTypes *[]string  `json:"activity_types"`
	Date          *time.Time `json:"date"`
	Until         *time.Time `json:"until"`
	After         *time.Time `json:"after"`
	Direction     *string    `json:"direction"`
	PageSize      *int       `json:"page_size"`
}

type Side string

const (
	Buy  Side = "buy"
	Sell Side = "sell"
)

type OrderType string

const (
	Market       OrderType = "market"
	Limit        OrderType = "limit"
	Stop         OrderType = "stop"
	StopLimit    OrderType = "stop_limit"
	TrailingStop OrderType = "trailing_stop"
)

type OrderClass string

const (
	Bracket OrderClass = "bracket"
	Oto     OrderClass = "oto"
	Oco     OrderClass = "oco"
	Simple  OrderClass = "simple"
)

type TimeInForce string

const (
	Day TimeInForce = "day"
	GTC TimeInForce = "gtc"
	OPG TimeInForce = "opg"
	IOC TimeInForce = "ioc"
	FOK TimeInForce = "fok"
	GTX TimeInForce = "gtx"
	GTD TimeInForce = "gtd"
	CLS TimeInForce = "cls"
)

type DtbpCheck string

const (
	Entry DtbpCheck = "entry"
	Exit  DtbpCheck = "exit"
	Both  DtbpCheck = "both"
)

type TradeConfirmEmail string

const (
	None TradeConfirmEmail = "none"
	All  TradeConfirmEmail = "all"
)

type RangeFreq string

const (
	Min1  RangeFreq = "1Min"
	Min5  RangeFreq = "5Min"
	Min15 RangeFreq = "15Min"
	Hour1 RangeFreq = "1H"
	Day1  RangeFreq = "1D"
)

// stream

// ClientMsg is the standard message sent by clients of the stream interface
type ClientMsg struct {
	Action string      `json:"action" msgpack:"action"`
	Data   interface{} `json:"data" msgpack:"data"`
}

// ServerMsg is the standard message sent by the server to update clients
// of the stream interface
type ServerMsg struct {
	Stream string      `json:"stream" msgpack:"stream"`
	Data   interface{} `json:"data"`
}

type TradeUpdate struct {
	Event string `json:"event"`
	Order Order  `json:"order"`
}

type StreamAgg struct {
	Event             string  `json:"ev"`
	Symbol            string  `json:"T"`
	Open              float32 `json:"o"`
	High              float32 `json:"h"`
	Low               float32 `json:"l"`
	Close             float32 `json:"c"`
	Volume            int32   `json:"v"`
	Start             int64   `json:"s"`
	End               int64   `json:"e"`
	OpenPrice         float32 `json:"op"`
	AccumulatedVolume int32   `json:"av"`
	VWAP              float32 `json:"vw"`
}

func (s *StreamAgg) Time() time.Time {
	// milliseconds
	return time.Unix(0, s.Start*1e6)
}

type StreamQuote struct {
	Event       string  `json:"ev"`
	Symbol      string  `json:"T"`
	BidPrice    float32 `json:"p"`
	BidSize     int32   `json:"s"`
	BidExchange int     `json:"x"`
	AskPrice    float32 `json:"P"`
	AskSize     int32   `json:"S"`
	AskExchange int     `json:"X"`
	Timestamp   int64   `json:"t"`
}

func (s *StreamQuote) Time() time.Time {
	// nanoseconds
	return time.Unix(0, s.Timestamp)
}

type StreamTrade struct {
	Event      string  `json:"ev"`
	Symbol     string  `json:"T"`
	TradeID    string  `json:"i"`
	Exchange   int     `json:"x"`
	Price      float32 `json:"p"`
	Size       int32   `json:"s"`
	Timestamp  int64   `json:"t"`
	Conditions []int   `json:"c"`
	TapeID     int     `json:"z"`
}

func (s *StreamTrade) Time() time.Time {
	// nanoseconds
	return time.Unix(0, s.Timestamp)
}
