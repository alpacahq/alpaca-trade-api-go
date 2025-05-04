package stream

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoSubscribeCallNecessary(t *testing.T) {
	tests := []struct {
		sub      subscriptions
		expected bool
	}{
		{sub: subscriptions{}, expected: true},
		{sub: subscriptions{trades: []string{}}, expected: true},
		{sub: subscriptions{trades: []string{"TEST"}}, expected: false},
		{sub: subscriptions{quotes: []string{"TEST"}}, expected: false},
		{sub: subscriptions{bars: []string{"TEST"}}, expected: false},
		{sub: subscriptions{dailyBars: []string{"TEST"}}, expected: false},
		{sub: subscriptions{statuses: []string{"TEST"}}, expected: false},
		{sub: subscriptions{imbalances: []string{"TEST"}}, expected: false},
		{sub: subscriptions{lulds: []string{"TEST"}}, expected: false},
		{sub: subscriptions{news: []string{"TEST"}}, expected: false},
		{sub: subscriptions{
			trades: []string{"TEST"}, quotes: []string{"TEST"}, bars: []string{"TEST"},
		}, expected: false},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.sub.noSubscribeCallNecessary())
	}
}
