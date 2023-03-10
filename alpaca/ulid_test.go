package alpaca

import (
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
)

func TestIsULIDZero(t *testing.T) {
	assert := assert.New(t)

	epochInMS := uint64(time.Now().Unix() * 1000)
	assert.False(isULIDZero(ulid.MustNew(epochInMS, ulid.DefaultEntropy())))

	var zeroULID ulid.ULID
	assert.True(isULIDZero(zeroULID))
}
