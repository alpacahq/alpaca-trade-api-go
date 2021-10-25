package marketdata

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestADTV(t *testing.T) {
	ind := NewIndicators(IndicatorsOpts{}).(*indicators)
	start := time.Date(2021, 10, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2021, 10, 24, 0, 0, 0, 0, time.UTC)
	ind.getBarsAsync = func(symbol string, params GetBarsParams) <-chan BarItem {
		assert.Equal(t, "AAPL", symbol)
		assert.Equal(t, start, params.Start)
		assert.Equal(t, end, params.End)
		ch := make(chan BarItem)
		go func() {
			defer close(ch)
			for _, bar := range []Bar{
				{Volume: 1},
				{Volume: 5},
				{Volume: 4},
				{Volume: 9},
			} {
				ch <- BarItem{Bar: bar}
			}
		}()
		return ch
	}
	got, err := ind.ADTV("AAPL", ADTVParams{Start: start, End: end})
	assert.NoError(t, err)
	assert.EqualValues(t, 4, got.Days)
	assert.EqualValues(t, 4.75, got.AverageVolume)

	t.Run("no bars", func(t *testing.T) {
		ind.getBarsAsync = func(symbol string, params GetBarsParams) <-chan BarItem {
			ch := make(chan BarItem)
			close(ch)
			return ch
		}
		got, err := ind.ADTV("MSFT", ADTVParams{Start: start, End: end})
		assert.NoError(t, err)
		assert.EqualValues(t, 0, got.Days)
		assert.EqualValues(t, 0, got.AverageVolume)
	})

	t.Run("error", func(t *testing.T) {
		ind.getBarsAsync = func(symbol string, params GetBarsParams) <-chan BarItem {
			assert.Equal(t, "IBM", symbol)
			ch := make(chan BarItem)
			go func() {
				defer close(ch)
				ch <- BarItem{Error: fmt.Errorf("something went wrong")}
			}()
			return ch
		}
		got, err := ind.ADTV("IBM", ADTVParams{Start: start, End: end})
		assert.Error(t, err)
		assert.Nil(t, got)
	})
}
