package stream

import (
	"fmt"
	"testing"

	"github.com/alpacahq/alpaca-trade-api-go/alpaca"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type StreamTestSuite struct {
	suite.Suite
	alp, poly *MockStream
}

func TestStreamTestSuite(t *testing.T) {
	suite.Run(t, new(StreamTestSuite))
}

func (s *StreamTestSuite) SetupSuite() {
	s.alp = &MockStream{}
	s.poly = &MockStream{}
	u = &Unified{
		alpaca: s.alp,
		data:   s.poly,
	}
}

func (s *StreamTestSuite) TestStream() {
	h := func(msg interface{}) {}

	// successful
	assert.Nil(s.T(), Register(alpaca.TradeUpdates, h))
	assert.Nil(s.T(), Register(alpaca.AccountUpdates, h))
	assert.Nil(s.T(), Register("T.*", h))
	assert.Nil(s.T(), Close())
	assert.Nil(s.T(), Deregister(alpaca.TradeUpdates))
	assert.Nil(s.T(), Deregister(alpaca.AccountUpdates))
	assert.Nil(s.T(), Deregister("T.*"))

	// failure
	s.alp.fail = true
	assert.NotNil(s.T(), Register(alpaca.TradeUpdates, h))
	assert.NotNil(s.T(), Close())
	assert.NotNil(s.T(), Deregister(alpaca.TradeUpdates))
}

type MockStream struct {
	fail bool
}

func (ms *MockStream) Subscribe(key string, handler func(msg interface{})) error {
	if ms.fail {
		return fmt.Errorf("failed to subscribe")
	}

	return nil
}

func (ms *MockStream) Unsubscribe(key string) error {
	if ms.fail {
		return fmt.Errorf("failed to unsubscribe")
	}

	return nil
}

func (ms *MockStream) Close() error {
	if ms.fail {
		return fmt.Errorf("failed to close")
	}

	return nil
}
