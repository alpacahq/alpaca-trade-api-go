package alpaca

import (
	"bufio"
	"bytes"
	"encoding/json"
	"testing"
)

// Benchmark for parsing a streaming trade update message.
func BenchmarkStreamTradeUpdateParsing(b *testing.B) {
	// Use double quotes so \n is a real newline.
	msg := []byte("data: {\"execution_id\":\"benchmark\",\"at\":\"2024-01-01T00:00:00Z\"}\n")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := bufio.NewReader(bytes.NewReader(msg))
		line, err := reader.ReadBytes('\n')
		if err != nil {
			b.Fatal(err)
		}
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		line = line[len("data: "):]
		var tu TradeUpdate
		if err := json.Unmarshal(line, &tu); err != nil {
			b.Fatal(err)
		}
	}
}
