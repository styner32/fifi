package snapshot

import (
	"context"
	"fmt"

	"github.com/fifi/internal/external/yahoo"
)

type GlobalSection struct {
	Quotes map[string]yahoo.Quote
	Reason string
}

func collectGlobal(ctx context.Context, client YahooQuotes) (*GlobalSection, error) {
	if client == nil {
		return nil, fmt.Errorf("yahoo dependency is nil")
	}
	quotes, err := client.GetQuotes(ctx, []string{"^N225", "NQ=F", "CL=F", "BTC-USD", "KRW=X"})
	quotes = cleanQuotes(quotes)
	if len(quotes) == 0 {
		if err == nil {
			err = fmt.Errorf("no valid quotes")
		}
		return nil, err
	}
	section := &GlobalSection{Quotes: quotes}
	if err != nil {
		section.Reason = err.Error()
	}
	return section, nil
}
