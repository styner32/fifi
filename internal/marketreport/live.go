package marketreport

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/collector/premarket"
	"github.com/fifi/internal/collector/pulse"
	"github.com/fifi/internal/collector/snapshot"
	"github.com/fifi/internal/domesticfutureoption"
	"github.com/fifi/internal/domesticstock"
	"github.com/fifi/internal/external/dataapi"
	"github.com/fifi/internal/external/kofia"
	"github.com/fifi/internal/external/naver"
	"github.com/fifi/internal/external/yahoo"
	"github.com/fifi/internal/market/facts"
)

func liveCollectors(config Config) Collectors {
	return Collectors{
		Premarket: func(ctx context.Context, opts premarket.Options) (*premarket.PremarketReport, error) {
			client, authErr := kisClient(ctx)
			httpClient := &http.Client{Timeout: 15 * time.Second}
			var stock premarket.DomesticStock
			if authErr == nil {
				stock = domesticstock.NewService(client)
				httpClient = client.Client
			}
			r := premarket.Collect(ctx, premarket.Deps{Stock: stock, Yahoo: yahoo.NewClient(httpClient, yahoo.Config{UserAgent: os.Getenv("USER_AGENT")}), Naver: naver.NewClient(httpClient, os.Getenv("USER_AGENT")), KOFIA: kofia.NewCachedClient(env("KOFIA_CACHE_DIR", ".cache"), os.Getenv("USER_AGENT")), Clock: config.Clock, StoreDir: opts.StoreDir, DailyPrices: dataapi.New(os.Getenv("DATA_API_KEY")), Calendar: &premarket.OfficialCalendar{Client: httpClient, UserAgent: os.Getenv("USER_AGENT")}}, opts)
			if authErr != nil {
				r.Errors["kis"] = authErr.Error()
			}
			return r, nil
		},
		Pulse: func(ctx context.Context, opts pulse.Options) (*pulse.Pulse, error) {
			client, err := kisClient(ctx)
			if err != nil {
				return nil, err
			}
			return pulse.Collect(ctx, pulse.Deps{Stock: domesticstock.NewService(client), Future: domesticfutureoption.NewService(client), Yahoo: yahoo.NewClient(client.Client, yahoo.Config{UserAgent: os.Getenv("USER_AGENT")}), Naver: naver.NewClient(client.Client, os.Getenv("USER_AGENT")), Clock: config.Clock, StoreDir: opts.StoreDir}, opts), nil
		},
		Snapshot: func(ctx context.Context, opts snapshot.Options) (*snapshot.Snapshot, error) {
			client, err := kisClient(ctx)
			if err != nil {
				return nil, err
			}
			store, storeErr := facts.NewStoreWithContext(ctx, os.Getenv("DATABASE_URL"))
			if storeErr != nil {
				store = facts.NewNullStore()
			}
			if closer, ok := store.(interface{ Close() error }); ok {
				defer closer.Close()
			}
			r := snapshot.Collect(ctx, snapshot.Deps{Clock: config.Clock, DomesticStock: domesticstock.NewService(client), DomesticFuture: domesticfutureoption.NewService(client), Yahoo: yahoo.NewClient(client.Client, yahoo.Config{UserAgent: os.Getenv("USER_AGENT")}), Naver: naver.NewClient(client.Client, os.Getenv("USER_AGENT")), KOFIA: kofia.NewCachedClient(env("KOFIA_CACHE_DIR", ".cache"), os.Getenv("USER_AGENT")), Facts: store}, opts)
			if storeErr != nil {
				r.Errors["facts"] = fmt.Errorf("market observation database unavailable; stored fallback disabled")
			}
			return r, nil
		},
	}
}

func kisClient(ctx context.Context) (*auth.KIClient, error) {
	key, secret := strings.TrimSpace(os.Getenv("APP_KEY")), strings.TrimSpace(os.Getenv("APP_SECRET"))
	if key == "" || secret == "" {
		return nil, fmt.Errorf("APP_KEY and APP_SECRET are required")
	}
	client := auth.NewKIClient(key, secret, "https://openapi.koreainvestment.com:9443", os.Getenv("USER_AGENT"))
	client.Client = &http.Client{Timeout: 15 * time.Second}
	client.SetTokenCachePath(env("AUTH_TOKEN_FILE", ".auth_token.json"))
	if _, err := client.EnsureAuthToken(ctx); err != nil {
		return nil, fmt.Errorf("KIS authentication failed: %w", err)
	}
	return client, nil
}
