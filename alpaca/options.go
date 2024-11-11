package alpaca

import (
	"cloud.google.com/go/civil"
	"github.com/shopspring/decimal"
)

//go:generate go install github.com/mailru/easyjson/...@v0.7.7
//go:generate easyjson -all -snake_case $GOFILE

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
	Type                 DeliverableType           `json:"type"`
	Symbol               string                    `json:"symbol"`
	AssetID              *string                   `json:"asset_id,omitempty"`
	Amount               string                    `json:"amount"`
	AllocationPercentage string                    `json:"allocation_percentage"`
	SettlementType       DeliverableSettlementType `json:"settlement_method"`
	DelayedSettlement    bool                      `json:"delayed_settlement"`
}

type OptionContract struct {
	ID                string              `json:"id"`
	Symbol            string              `json:"symbol"`
	Name              string              `json:"name"`
	Status            string              `json:"status"`
	Tradable          bool                `json:"tradable"`
	ExpirationDate    civil.Date          `json:"expiration_date"`
	RootSymbol        *string             `json:"root_symbol,omitempty"`
	UnderlyingSymbol  string              `json:"underlying_symbol"`
	UnderlyingAssetID string              `json:"underlying_assest_id"`
	Type              OptionType          `json:"type"`
	Style             OptionStyle         `json:"style"`
	StrikePrice       decimal.Decimal     `json:"strike_price"`
	Multiplier        string              `json:"multiplier"`
	Size              string              `json:"size"`
	OpenInterest      *string             `json:"open_interest"`
	OpenInterestDate  *civil.Date         `json:"open_interest_date,omitempty"`
	ClosePrice        *decimal.Decimal    `json:"close_price,omitempty"`
	ClosePriceDate    *civil.Date         `json:"close_price_date,omitempty"`
	Deliverables      []OptionDeliverable `json:"deliverables,omitempty"`
}

type optionContractsResponse struct {
	OptionContracts []OptionContract `json:"option_contracts"`
	NextPageToken   *string          `json:"next_page_token,omitempty"`
}
