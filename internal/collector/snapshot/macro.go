package snapshot

import (
	"context"
	"fmt"

	"github.com/fifi/internal/external/yahoo"
)

type MacroSection struct {
	Quotes              map[string]yahoo.Quote
	USDKRWMonthStart    *float64
	USDKRWMonthStartPct *float64
	Reason              string
}

func collectMacro(ctx context.Context, client YahooQuotes, opts Options) (*MacroSection, error) {
	if client == nil {
		return nil, fmt.Errorf("yahoo dependency is nil")
	}
	quotes, err := client.GetQuotes(ctx, []string{"KRW=X", "CL=F", "^TNX"})
	if len(quotes) == 0 {
		return nil, err
	}
	section := &MacroSection{Quotes: quotes, USDKRWMonthStart: opts.USDKRWMonthStart}
	if opts.USDKRWMonthStart != nil {
		if krw, ok := quotes["KRW=X"]; ok && *opts.USDKRWMonthStart > 0 {
			section.USDKRWMonthStartPct = ptr((krw.Price / *opts.USDKRWMonthStart - 1) * 100)
		}
	}
	if err != nil {
		section.Reason = err.Error()
	}
	return section, nil
}
