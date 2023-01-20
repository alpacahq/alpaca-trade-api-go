package alpaca

import (
	json "encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/civil"
	"github.com/shopspring/decimal"

	// Required for easyjson generation
	_ "github.com/mailru/easyjson/gen"
)

//go:generate go install github.com/mailru/easyjson/...@v0.7.7
//go:generate easyjson -all -lower_camel_case $GOFILE

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
	ReplacedBy     *string          `json:"replaced_by"`
	AssetID        string           `json:"asset_id"`
	Symbol         string           `json:"symbol"`
	Class          string           `json:"asset_class"`
	OrderClass     OrderClass       `json:"order_class"`
	Qty            *decimal.Decimal `json:"qty"`
	Notional       *decimal.Decimal `json:"notional"`
	FilledQty      decimal.Decimal  `json:"filled_qty"`
	Type           OrderType        `json:"type"`
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
	AssetID        string           `json:"asset_id"`
	Symbol         string           `json:"symbol"`
	Exchange       string           `json:"exchange"`
	Class          string           `json:"asset_class"`
	AccountID      string           `json:"account_id"`
	EntryPrice     decimal.Decimal  `json:"avg_entry_price"`
	Qty            decimal.Decimal  `json:"qty"`
	Side           string           `json:"side"`
	MarketValue    *decimal.Decimal `json:"market_value"`
	CostBasis      decimal.Decimal  `json:"cost_basis"`
	UnrealizedPL   *decimal.Decimal `json:"unrealized_pl"`
	UnrealizedPLPC *decimal.Decimal `json:"unrealized_plpc"`
	CurrentPrice   *decimal.Decimal `json:"current_price"`
	LastdayPrice   *decimal.Decimal `json:"lastday_price"`
	ChangeToday    *decimal.Decimal `json:"change_today"`
}

type Asset struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Exchange     string `json:"exchange"`
	Class        string `json:"class"`
	Symbol       string `json:"symbol"`
	Status       string `json:"status"`
	Tradable     bool   `json:"tradable"`
	Marginable   bool   `json:"marginable"`
	Shortable    bool   `json:"shortable"`
	EasyToBorrow bool   `json:"easy_to_borrow"`
	Fractionable bool   `json:"fractionable"`
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
	Date            civil.Date      `json:"date"`
	NetAmount       decimal.Decimal `json:"net_amount"`
	Description     string          `json:"description"`
	PerShareAmount  decimal.Decimal `json:"per_share_amount"`
}

type PortfolioHistory struct {
	BaseValue     decimal.Decimal   `json:"base_value"`
	Equity        []decimal.Decimal `json:"equity"`
	ProfitLoss    []decimal.Decimal `json:"profit_loss"`
	ProfitLossPct []decimal.Decimal `json:"profit_loss_pct"`
	Timeframe     TimeFrame         `json:"timeframe"`
	Timestamp     []int64           `json:"timestamp"`
}

type OrderAttributes struct {
	TakeProfitLimitPrice *decimal.Decimal `json:"take_profit_limit_price,omitempty"`
	StopLossStopPrice    *decimal.Decimal `json:"stop_loss_stop_price,omitempty"`
	StopLossLimitPrice   *decimal.Decimal `json:"stop_loss_limit_price,omitempty"`
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

type TimeFrame string

const (
	Min1  TimeFrame = "1Min"
	Min5  TimeFrame = "5Min"
	Min15 TimeFrame = "15Min"
	Hour1 TimeFrame = "1H"
	Day1  TimeFrame = "1D"
)

// stream

type TradeUpdate struct {
	Event       string           `json:"event"`
	ExecutionID string           `json:"execution_id"`
	Order       Order            `json:"order"`
	PositionQty *decimal.Decimal `json:"position_qty"`
	Price       *decimal.Decimal `json:"price"`
	Qty         *decimal.Decimal `json:"qty"`
	Timestamp   *time.Time       `json:"timestamp"`
}

type DateType string

const (
	DeclarationDate DateType = "declaration_date"
	RecordDate      DateType = "record_date"
	ExDate          DateType = "ex_date"
	PayableDate     DateType = "payable_date"
)

type Announcement struct {
	ID                      string `json:"id"`
	CorporateActionsID      string `json:"corporate_actions_id"`
	CAType                  string `json:"ca_type"`
	CASubType               string `json:"ca_sub_type"`
	InitiatingSymbol        string `json:"initiating_symbol"`
	InitiatingOriginalCusip string `json:"initiating_original_cusip"`
	TargetSymbol            string `json:"target_symbol"`
	TargetOriginalCusip     string `json:"target_original_cusip"`
	DeclarationDate         string `json:"declaration_date"`
	ExpirationDate          string `json:"expiration_date"`
	RecordDate              string `json:"record_date"`
	PayableDate             string `json:"payable_date"`
	Cash                    string `json:"cash"`
	OldRate                 string `json:"old_rate"`
	NewRate                 string `json:"new_rate"`
}

type Watchlist struct {
	AccountID string  `json:"account_id"`
	ID        string  `json:"id"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
	Name      string  `json:"name"`
	Assets    []Asset `json:"assets"`
}

type CreateWatchlistRequest struct {
	Name    string   `json:"name"`
	Symbols []string `json:"symbols"`
}

type UpdateWatchlistRequest struct {
	Name    string   `json:"name"`
	Symbols []string `json:"symbols"`
}

type AddSymbolToWatchlistRequest struct {
	Symbol string `json:"symbol"`
}

type RemoveSymbolFromWatchlistRequest struct {
	Symbol string `json:"symbol"`
}

// APIError wraps the detailed code and message supplied
// by Alpaca's API for debugging purposes
type APIError struct {
	StatusCode int    `json:"-"`
	Code       int    `json:"code"`
	Message    string `json:"message"`
	Body       string `json:"-"`
}

func APIErrorFromResponse(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var apiErr APIError
	if err := json.Unmarshal(body, &apiErr); err != nil {
		// If the error is not in our JSON format, we simply return the HTTP response
		return fmt.Errorf("%s (HTTP %d)", body, resp.StatusCode)
	}
	apiErr.StatusCode = resp.StatusCode
	apiErr.Body = strings.TrimSpace(string(body))
	return &apiErr
}

func (e *APIError) Error() string {
	if e.Code != 0 {
		return fmt.Sprintf("%s (HTTP %d, Code %d)", e.Message, e.StatusCode, e.Code)
	}
	return fmt.Sprintf("%s (HTTP %d)", e.Message, e.StatusCode)
}
