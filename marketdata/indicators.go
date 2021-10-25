package marketdata

import "time"

// TechnicalIndicators can be used to calculate technical indicators.
type TechnicalIndicators interface {
	// ADTV calculates the average daily trading volume.
	ADTV(symbol string, params ADTVParams) (*ADTV, error)
}

// ADTVParams contains optional parameters for getting Average Daily Trading Volume
type ADTVParams struct {
	// Start is the inclusive beginning of the interval
	Start time.Time
	// End is the inclusive end of the interval
	End time.Time
}

type indicators struct {
	c Client

	// mockable functions
	getBarsAsync func(symbol string, params GetBarsParams) <-chan BarItem
}

type IndicatorsOpts struct {
	Client Client
}

func NewIndicators(opts IndicatorsOpts) TechnicalIndicators {
	var c Client
	if opts.Client == nil {
		c = DefaultClient
	}
	return &indicators{
		c:            c,
		getBarsAsync: c.GetBarsAsync,
	}
}

// Indicators can be used to query technical indicators using the default client.
var Indicators = NewIndicators(IndicatorsOpts{})

// ADTV calculates the average daily trading volume.
func (i *indicators) ADTV(symbol string, params ADTVParams) (*ADTV, error) {
	var (
		totalVolume uint64
		count       int
	)
	for item := range i.getBarsAsync(symbol, GetBarsParams{
		Start: params.Start,
		End:   params.End,
	}) {
		if item.Error != nil {
			return nil, item.Error
		}
		totalVolume += item.Bar.Volume
		count++
	}
	if count == 0 {
		return &ADTV{}, nil
	}
	return &ADTV{
		AverageVolume: float64(totalVolume) / float64(count),
		Days:          count,
	}, nil
}

// ADTV is the average daily trading volume. It also contains the number of trading days
// the average contains.
type ADTV struct {
	AverageVolume float64
	Days          int
}
