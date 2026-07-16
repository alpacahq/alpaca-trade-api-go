package alpaca

import (
	json "encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/civil"
	// Required for easyjson generation
	_ "github.com/mailru/easyjson/gen"
	"github.com/shopspring/decimal"
)

//go:generate go install github.com/mailru/easyjson/...@v0.7.7
//go:generate easyjson -all -snake_case $GOFILE

type Account struct {
	ID                    string          `json:"id"`
	AccountNumber         string          `json:"account_number"`
	Status                string          `json:"status"`
	CryptoStatus          string          `json:"crypto_status"`
	Currency              string          `json:"currency"`
	BuyingPower           decimal.Decimal `json:"buying_power"`
	RegTBuyingPower       decimal.Decimal `json:"regt_buying_power"`
	DaytradingBuyingPower decimal.Decimal `json:"daytrading_buying_power"`
	EffectiveBuyingPower  decimal.Decimal `json:"effective_buying_power"`
	NonMarginBuyingPower  decimal.Decimal `json:"non_marginable_buying_power"`
	BodDtbp               decimal.Decimal `json:"bod_dtbp"`
	Cash                  decimal.Decimal `json:"cash"`
	AccruedFees           decimal.Decimal `json:"accrued_fees"`
	PortfolioValue        decimal.Decimal `json:"portfolio_value"`
	PatternDayTrader      bool            `json:"pattern_day_trader"`
	TradingBlocked        bool            `json:"trading_blocked"`
	TransfersBlocked      bool            `json:"transfers_blocked"`
	AccountBlocked        bool            `json:"account_blocked"`
	ShortingEnabled       bool            `json:"shorting_enabled"`
	TradeSuspendedByUser  bool            `json:"trade_suspended_by_user"`
	CreatedAt             time.Time       `json:"created_at"`
	Multiplier            decimal.Decimal `json:"multiplier"`
	Equity                decimal.Decimal `json:"equity"`
	LastEquity            decimal.Decimal `json:"last_equity"`
	LongMarketValue       decimal.Decimal `json:"long_market_value"`
	ShortMarketValue      decimal.Decimal `json:"short_market_value"`
	PositionMarketValue   decimal.Decimal `json:"position_market_value"`
	InitialMargin         decimal.Decimal `json:"initial_margin"`
	MaintenanceMargin     decimal.Decimal `json:"maintenance_margin"`
	LastMaintenanceMargin decimal.Decimal `json:"last_maintenance_margin"`
	SMA                   decimal.Decimal `json:"sma"`
	DaytradeCount         int64           `json:"daytrade_count"`
	CryptoTier            int             `json:"crypto_tier"`
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
	ReplacedBy     *string          `json:"replaced_by"`
	Replaces       *string          `json:"replaces"`
	AssetID        string           `json:"asset_id"`
	Symbol         string           `json:"symbol"`
	AssetClass     AssetClass       `json:"asset_class"`
	OrderClass     OrderClass       `json:"order_class"`
	Type           OrderType        `json:"type"`
	Side           Side             `json:"side"`
	PositionIntent PositionIntent   `json:"position_intent"`
	TimeInForce    TimeInForce      `json:"time_in_force"`
	Status         string           `json:"status"`
	Notional       *decimal.Decimal `json:"notional"`
	Qty            *decimal.Decimal `json:"qty"`
	FilledQty      decimal.Decimal  `json:"filled_qty"`
	FilledAvgPrice *decimal.Decimal `json:"filled_avg_price"`
	LimitPrice     *decimal.Decimal `json:"limit_price"`
	StopPrice      *decimal.Decimal `json:"stop_price"`
	TrailPrice     *decimal.Decimal `json:"trail_price"`
	TrailPercent   *decimal.Decimal `json:"trail_percent"`
	HWM            *decimal.Decimal `json:"hwm"`
	ExtendedHours  bool             `json:"extended_hours"`
	RatioQty       *decimal.Decimal `json:"ratio_qty"`
	Legs           []Order          `json:"legs"`
}

//easyjson:json
type orderSlice []Order

type Position struct {
	AssetID                string           `json:"asset_id"`
	Symbol                 string           `json:"symbol"`
	Exchange               string           `json:"exchange"`
	AssetClass             AssetClass       `json:"asset_class"`
	AssetMarginable        bool             `json:"asset_marginable"`
	Qty                    decimal.Decimal  `json:"qty"`
	QtyAvailable           decimal.Decimal  `json:"qty_available"`
	AvgEntryPrice          decimal.Decimal  `json:"avg_entry_price"`
	Side                   string           `json:"side"`
	MarketValue            *decimal.Decimal `json:"market_value"`
	CostBasis              decimal.Decimal  `json:"cost_basis"`
	UnrealizedPL           *decimal.Decimal `json:"unrealized_pl"`
	UnrealizedPLPC         *decimal.Decimal `json:"unrealized_plpc"`
	UnrealizedIntradayPL   *decimal.Decimal `json:"unrealized_intraday_pl"`
	UnrealizedIntradayPLPC *decimal.Decimal `json:"unrealized_intraday_plpc"`
	CurrentPrice           *decimal.Decimal `json:"current_price"`
	LastdayPrice           *decimal.Decimal `json:"lastday_price"`
	ChangeToday            *decimal.Decimal `json:"change_today"`
}

//easyjson:json
type positionSlice []Position

type Asset struct {
	ID                           string      `json:"id"`
	Class                        AssetClass  `json:"class"`
	Exchange                     string      `json:"exchange"`
	Symbol                       string      `json:"symbol"`
	Name                         string      `json:"name"`
	Status                       AssetStatus `json:"status"`
	Tradable                     bool        `json:"tradable"`
	Marginable                   bool        `json:"marginable"`
	MaintenanceMarginRequirement uint        `json:"maintenance_margin_requirement"`
	Shortable                    bool        `json:"shortable"`
	EasyToBorrow                 bool        `json:"easy_to_borrow"`
	Fractionable                 bool        `json:"fractionable"`
	Attributes                   []string    `json:"attributes"`
}

//easyjson:json
type assetSlice []Asset

type AssetStatus string

const (
	AssetActive   AssetStatus = "active"
	AssetInactive AssetStatus = "inactive"
)

type AssetClass string

const (
	USEquity AssetClass = "us_equity"
	Crypto   AssetClass = "crypto"
)

type CalendarDay struct {
	Date  string `json:"date"`
	Open  string `json:"open"`
	Close string `json:"close"`
}

//easyjson:json
type calendarDaySlice []CalendarDay

type Clock struct {
	Timestamp time.Time `json:"timestamp"`
	IsOpen    bool      `json:"is_open"`
	NextOpen  time.Time `json:"next_open"`
	NextClose time.Time `json:"next_close"`
}

type AccountConfigurations struct {
	DTBPCheck            DTBPCheck         `json:"dtbp_check"`
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
	OrderID         string          `json:"order_id"`
	OrderStatus     string          `json:"order_status"`
	Status          string          `json:"status"`
}

//easyjson:json
type accountSlice []AccountActivity

type PortfolioHistory struct {
	BaseValue     decimal.Decimal   `json:"base_value"`
	Equity        []decimal.Decimal `json:"equity"`
	ProfitLoss    []decimal.Decimal `json:"profit_loss"`
	ProfitLossPct []decimal.Decimal `json:"profit_loss_pct"`
	Timeframe     TimeFrame         `json:"timeframe"`
	Timestamp     []int64           `json:"timestamp"`
}

type Side string

const (
	Buy  Side = "buy"
	Sell Side = "sell"
)

type PositionIntent string

const (
	BuyToOpen   PositionIntent = "buy_to_open"
	BuyToClose  PositionIntent = "buy_to_close"
	SellToOpen  PositionIntent = "sell_to_open"
	SellToClose PositionIntent = "sell_to_close"
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
	OTO     OrderClass = "oto"
	OCO     OrderClass = "oco"
	Simple  OrderClass = "simple"
	MLeg    OrderClass = "mleg"
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

type DTBPCheck string

const (
	Entry DTBPCheck = "entry"
	Exit  DTBPCheck = "exit"
	Both  DTBPCheck = "both"
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
	At          time.Time        `json:"at"`
	Event       string           `json:"event"`
	EventID     string           `json:"event_id"`
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

//easyjson:json
type announcementSlice []Announcement

type Watchlist struct {
	AccountID string  `json:"account_id"`
	ID        string  `json:"id"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
	Name      string  `json:"name"`
	Assets    []Asset `json:"assets"`
}

//easyjson:json
type watchlistSlice []Watchlist

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

//easyjson:json
type closeAllPositionsSlice []closeAllPositionsResponse

type closeAllPositionsResponse struct {
	Symbol string          `json:"symbol"`
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body,omitempty"`
}

type OptionStatus string

const (
	OptionStatusActive   OptionStatus = "active"
	OptionStatusInactive OptionStatus = "inactive"
)

type OptionType string

const (
	OptionTypeCall OptionType = "call"
	OptionTypePut  OptionType = "put"
)

type OptionStyle string

const (
	OptionStyleAmerican OptionStyle = "american"
	OptionStyleEuropean OptionStyle = "european"
)

type DeliverableType string

const (
	DeliverableTypeCash   DeliverableType = "cash"
	DeliverableTypeEquity DeliverableType = "equity"
)

type DeliverableSettlementType string

const (
	DeliverableSettlementTypeT0 DeliverableSettlementType = "T+0"
	DeliverableSettlementTypeT1 DeliverableSettlementType = "T+1"
	DeliverableSettlementTypeT2 DeliverableSettlementType = "T+2"
	DeliverableSettlementTypeT3 DeliverableSettlementType = "T+3"
	DeliverableSettlementTypeT4 DeliverableSettlementType = "T+4"
	DeliverableSettlementTypeT5 DeliverableSettlementType = "T+5"
)

type DeliverableSettlementMethod string

const (
	DeliverableSettlementMethodBTOB DeliverableSettlementMethod = "BTOB"
	DeliverableSettlementMethodCADF DeliverableSettlementMethod = "CADF"
	DeliverableSettlementMethodCAFX DeliverableSettlementMethod = "CAFX"
	DeliverableSettlementMethodCCC  DeliverableSettlementMethod = "CCC"
)

type OptionDeliverable struct {
	Type                 DeliverableType             `json:"type"`
	Symbol               string                      `json:"symbol"`
	AssetID              *string                     `json:"asset_id,omitempty"`
	Amount               decimal.Decimal             `json:"amount"`
	AllocationPercentage decimal.Decimal             `json:"allocation_percentage"`
	SettlementType       DeliverableSettlementType   `json:"settlement_type"`
	SettlementMethod     DeliverableSettlementMethod `json:"settlement_method"`
	DelayedSettlement    bool                        `json:"delayed_settlement"`
}

type OptionContract struct {
	ID                string              `json:"id"`
	Symbol            string              `json:"symbol"`
	Name              string              `json:"name"`
	Status            OptionStatus        `json:"status"`
	Tradable          bool                `json:"tradable"`
	ExpirationDate    civil.Date          `json:"expiration_date"`
	RootSymbol        *string             `json:"root_symbol,omitempty"`
	UnderlyingSymbol  string              `json:"underlying_symbol"`
	UnderlyingAssetID string              `json:"underlying_asset_id"`
	Type              OptionType          `json:"type"`
	Style             OptionStyle         `json:"style"`
	StrikePrice       decimal.Decimal     `json:"strike_price"`
	Multiplier        decimal.Decimal     `json:"multiplier"`
	Size              decimal.Decimal     `json:"size"`
	OpenInterest      *decimal.Decimal    `json:"open_interest"`
	OpenInterestDate  *civil.Date         `json:"open_interest_date,omitempty"`
	ClosePrice        *decimal.Decimal    `json:"close_price,omitempty"`
	ClosePriceDate    *civil.Date         `json:"close_price_date,omitempty"`
	Deliverables      []OptionDeliverable `json:"deliverables,omitempty"`
}

type optionContractsResponse struct {
	OptionContracts []OptionContract `json:"option_contracts"`
	NextPageToken   *string          `json:"next_page_token,omitempty"`
}
