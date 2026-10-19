package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/domesticstock"
	"github.com/fifi/internal/kst"
	"github.com/fifi/internal/market/calendar"
	"github.com/fifi/internal/market/facts"
	"github.com/fifi/internal/parse"
)

func collectMetrics(ctx context.Context, client *auth.KIClient, store facts.Store, force bool) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	now := kst.Now()
	if calendar.ResolveKRXSession(ctx, nil, now).ClosedDay() {
		log.Printf("[market-collector] weekend; skip tick")
		return
	}
	if _, err := client.EnsureAuthToken(ctx); err != nil {
		log.Printf("[market-collector] auth failed: %v", err)
		return
	}
	stock := domesticstock.NewService(client)
	session := calendar.ResolveKRXSession(ctx, stock, now)
	if !shouldCollect(session, force) {
		log.Printf("[market-collector] skip tick: %s", session.Label())
		return
	}
	log.Printf("[market-collector] tick: %s", session.Label())
	var wg sync.WaitGroup
	var mu sync.Mutex
	var observations []facts.Observation
	collect := func(metric, field, unit string, divisor float64, fetch func() (*auth.RESTResponse, error)) {
		defer wg.Done()
		response, err := fetch()
		if err != nil || response == nil || !response.IsOK() {
			log.Printf("[market-collector] %s unavailable (transport/business response)", metric)
			return
		}
		obs, err := sourceObservation(metric, field, unit, divisor, response.FirstRow("output"), time.Now())
		if err != nil {
			log.Printf("[market-collector] %s not accepted: %v", metric, err)
			return
		}
		mu.Lock()
		observations = append(observations, obs)
		mu.Unlock()
	}
	wg.Add(3)
	go collect("vkospi.value", "bstp_nmix_prpr", "index", 1, func() (*auth.RESTResponse, error) {
		code, err := stock.ResolveVKOSPICode(ctx, nil)
		if err != nil {
			return nil, err
		}
		return stock.InquireVKOSPIPrice(ctx, code)
	})
	go collect("kospi.index", "bstp_nmix_prpr", "index", 1, func() (*auth.RESTResponse, error) { return stock.InquireIndexPrice(ctx, "0001") })
	go collect("kosdaq.index", "bstp_nmix_prpr", "index", 1, func() (*auth.RESTResponse, error) { return stock.InquireIndexPrice(ctx, "1001") })
	// One market-flow call supplies all investors; fields are million KRW.
	wg.Add(1)
	go func() {
		defer wg.Done()
		response, err := stock.InquireInvestorTimeByMarket(ctx, "KSP", "0001")
		if err != nil || response == nil || !response.IsOK() {
			return
		}
		for _, investor := range []struct{ metric, field string }{{"foreign", "frgn_ntby_tr_pbmn"}, {"organ", "orgn_ntby_tr_pbmn"}, {"individual", "prsn_ntby_tr_pbmn"}} {
			obs, err := sourceObservation("kospi.flow."+investor.metric+"_eok", investor.field, "억원", 100, response.FirstRow("output"), time.Now())
			if err != nil {
				log.Printf("[market-collector] flow not accepted: %v", err)
				continue
			}
			mu.Lock()
			observations = append(observations, obs)
			mu.Unlock()
		}
	}()
	wg.Wait()
	if err := store.Put(ctx, observations...); err != nil {
		log.Printf("[market-collector] store failed: %v", err)
	} else {
		log.Printf("[market-collector] accepted %d source-timestamped observations", len(observations))
	}
}

func shouldCollect(session calendar.Session, force bool) bool {
	if session.ClosedDay() {
		return false
	}
	return force || session.Phase == "CONTINUOUS" || session.Phase == "CLOSING_AUCTION" || session.Phase == "POST_CLOSE"
}

func sourceObservation(metric, field, unit string, divisor float64, row map[string]any, retrieved time.Time) (facts.Observation, error) {
	value, ok := parse.Float(row[field])
	if !ok || math.IsNaN(value) || math.IsInf(value, 0) || divisor <= 0 {
		return facts.Observation{}, fmt.Errorf("invalid or missing %s", field)
	}
	var date, hour string
	for _, key := range []string{"stck_bsop_date", "bsop_date", "business_date"} {
		if s, ok := row[key].(string); ok && facts.NormalizeBusinessDate(s) != "" {
			date = facts.NormalizeBusinessDate(s)
			break
		}
	}
	for _, key := range []string{"stck_cntg_hour", "bsop_hour", "aspr_hour", "cntg_hour"} {
		if s, ok := row[key].(string); ok && len(strings.TrimSpace(s)) == 6 {
			hour = strings.TrimSpace(s)
			break
		}
	}
	if date == "" || hour == "" {
		return facts.Observation{}, fmt.Errorf("SOURCE_TIMESTAMP_MISSING")
	}
	observed, err := time.ParseInLocation("20060102150405", date+hour, kst.Location)
	if err != nil {
		return facts.Observation{}, fmt.Errorf("invalid source timestamp")
	}
	value /= divisor
	obs := facts.Observation{ProvenanceVersion: facts.CurrentProvenanceVersion, MetricID: metric, BusinessDate: date, ObservedAt: observed, RetrievedAt: retrieved,
		Value: &value, Unit: unit, Source: "KIS", SourceField: field, FreshnessStatus: "VALID", Raw: row}
	if !facts.AcceptedAt(obs, retrieved) {
		return facts.Observation{}, fmt.Errorf("future or invalid source observation")
	}
	return obs, nil
}
