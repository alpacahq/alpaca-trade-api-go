package alpaca

import "github.com/oklog/ulid/v2"

func isULIDZero(u ulid.ULID) bool {
	for _, b := range u.Bytes() {
		if b != 0 {
			return false
		}
	}

	return true
}
