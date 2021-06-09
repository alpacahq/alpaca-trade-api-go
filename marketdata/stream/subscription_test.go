package stream

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoSubscribeCallNecessary(t *testing.T) {
	var tests = []struct {
		trades    []string
		quotes    []string
		bars      []string
		dailyBars []string
		expected  bool
	}{
		{trades: nil, quotes: nil, bars: nil, expected: true},
		{trades: []string{"TEST"}, quotes: nil, bars: nil, expected: false},
		{trades: nil, quotes: []string{"TEST"}, bars: nil, expected: false},
		{trades: nil, quotes: nil, bars: []string{"TEST"}, expected: false},
		{trades: []string{"TEST"}, quotes: []string{"TEST"}, bars: []string{"TEST"}, expected: false},
		{dailyBars: []string{"TEST"}, expected: false},
	}

	for _, test := range tests {

		sub := subscriptions{
			trades:    test.trades,
			quotes:    test.quotes,
			bars:      test.bars,
			dailyBars: test.dailyBars,
		}
		assert.Equal(t, test.expected, sub.noSubscribeCallNecessary())
	}
}
