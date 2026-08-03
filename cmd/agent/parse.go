package main

import (
	"flag"
	"os"

	"github.com/fifi/internal/collector/snapshot"
)

func parseOptions(args []string) (snapshot.Options, error) {
	fs := flag.NewFlagSet("market-snapshot", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	date := fs.String("date", "", "business date YYYY-MM-DD or YYYYMMDD")
	sidecarStatus := fs.String("sidecar-status", env("MARKET_SNAPSHOT_SIDECAR_STATUS"), "triggered, not-triggered, unknown")
	sidecarTime := fs.String("sidecar-time", env("MARKET_SNAPSHOT_SIDECAR_TIME"), "sidecar trigger time")
	semi := fs.String("semiconductor-foreign-net-sell-eok", env("MARKET_SNAPSHOT_SEMICONDUCTOR_FOREIGN_NET_SELL_EOK"), "semiconductor foreign net sell in eok KRW")
	monthly := fs.String("monthly-foreign-net-sell-eok", env("MARKET_SNAPSHOT_MONTHLY_FOREIGN_NET_SELL_EOK"), "monthly foreign net sell in eok KRW")
	monthlyNote := fs.String("monthly-foreign-note", env("MARKET_SNAPSHOT_MONTHLY_FOREIGN_NOTE"), "monthly foreign flow note")
	holding := fs.String("foreign-holding-change-pp", env("MARKET_SNAPSHOT_FOREIGN_HOLDING_CHANGE_PP"), "foreign holding one-month change in percentage points")
	capRatio := fs.String("samsung-sk-hynix-cap-ratio", env("MARKET_SNAPSHOT_SAMSUNG_SK_HYNIX_CAP_RATIO"), "Samsung+SK Hynix KOSPI market cap ratio")
	usdkrwStart := fs.String("usdkrw-month-start", env("MARKET_SNAPSHOT_USDKRW_MONTH_START"), "month-start USD/KRW")
	if err := fs.Parse(args); err != nil {
		return snapshot.Options{}, err
	}
	return buildOptions(*date, *sidecarStatus, *sidecarTime, *semi, *monthly, *monthlyNote, *holding, *capRatio, *usdkrwStart)
}

func buildOptions(date, sidecarStatus, sidecarTime, semi, monthly, note, holding, capRatio, usdkrwStart string) (snapshot.Options, error) {
	normalizedDate, err := validateDate(date)
	if err != nil {
		return snapshot.Options{}, err
	}
	if err := validateSidecar(sidecarStatus); err != nil {
		return snapshot.Options{}, err
	}
	semiValue, err := parseOptionalFloat(semi, "semiconductor-foreign-net-sell-eok")
	if err != nil {
		return snapshot.Options{}, err
	}
	monthlyValue, err := parseOptionalFloat(monthly, "monthly-foreign-net-sell-eok")
	if err != nil {
		return snapshot.Options{}, err
	}
	holdingValue, err := parseOptionalFloat(holding, "foreign-holding-change-pp")
	if err != nil {
		return snapshot.Options{}, err
	}
	capRatioValue, err := parseOptionalFloat(capRatio, "samsung-sk-hynix-cap-ratio")
	if err != nil {
		return snapshot.Options{}, err
	}
	usdkrwStartValue, err := parseOptionalFloat(usdkrwStart, "usdkrw-month-start")
	if err != nil {
		return snapshot.Options{}, err
	}
	return snapshot.Options{
		Date: normalizedDate, SidecarStatus: sidecarStatus, SidecarTime: sidecarTime,
		SemiconductorForeignNetSellEok: semiValue,
		MonthlyForeignNetSellEok:       monthlyValue,
		MonthlyForeignNote:             note,
		ForeignHoldingChangePP:         holdingValue,
		SamsungSKHynixCapRatio:         capRatioValue,
		USDKRWMonthStart:               usdkrwStartValue,
	}, nil
}
