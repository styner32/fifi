package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/fifi/internal/auth"
	"github.com/fifi/internal/commodityfuture"
	"github.com/fifi/internal/companyanalysis"
	"github.com/fifi/internal/dcf"
	"github.com/fifi/internal/domesticfutureoption"
	"github.com/fifi/internal/domesticstock"
	"github.com/fifi/internal/overseasstock"
	"github.com/fifi/internal/quadwitching"
	"github.com/fifi/internal/shippingindex"
)

type appDeps struct {
	client             *auth.KIClient
	domesticStock      *domesticstock.Service
	domesticFuture     *domesticfutureoption.Service
	commodityFuture    *commodityfuture.Service
	overseasStock      *overseasstock.Service
	shippingIndex      *shippingindex.Service
	companyAnalysisSvc *companyanalysis.Service
}

type appDates struct {
	now                  time.Time
	today                string
	fromDate             string
	exchangeRateFromDate string
}

type domesticMarketState struct {
	businessDate          string
	marketTime            *auth.RESTResponse
	kospi                 *auth.RESTResponse
	programTrade          *auth.RESTResponse
	quadRunWindow         quadwitching.RunWindow
	shouldRunQuadWitching bool
}

func runApp() error {
	now := time.Now()
	cfg := loadAppConfig(now)
	dates := newAppDates(now)
	deps := newAppDeps(cfg)

	if err := authenticateAndRender(deps.client, cfg.tokenCachePath); err != nil {
		return err
	}

	printUsefulEndpoints()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pbrCtx, pbrCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer pbrCancel()

	marketState, err := runDomesticMarket(ctx, pbrCtx, cfg, deps, dates)
	if err != nil {
		return err
	}

	runQuadWitching(ctx, cfg, deps, marketState)

	if err := runGlobalMarkets(ctx, cfg, deps, dates); err != nil {
		return err
	}

	runCompanyAnalysis(ctx, cfg, deps, marketState.businessDate)

	if err := runDomesticValuation(ctx, cfg, deps, dates, marketState.businessDate); err != nil {
		return err
	}

	printClientMetricsSummary(deps.client.MetricsSnapshot())
	return nil
}

func newAppDeps(cfg appConfig) appDeps {
	client := auth.NewKIClient(cfg.appKey, cfg.secretKey, cfg.baseURL, cfg.userAgent)
	client.SetTokenCachePath(cfg.tokenCachePath)

	return appDeps{
		client:        client,
		domesticStock: domesticstock.NewService(client),
		domesticFuture: domesticfutureoption.NewService(
			client,
		),
		commodityFuture: commodityfuture.NewService(client.Client, commodityfuture.Config{
			ProviderOrder: cfg.commodityFutureProviderOrder,
			UserAgent:     cfg.commodityFutureUserAgent,
		}),
		overseasStock: overseasstock.NewService(client),
		shippingIndex: shippingindex.NewService(client),
		companyAnalysisSvc: companyanalysis.NewService(client.Client, companyanalysis.Config{
			SECUserAgent:             cfg.companyAnalysisSECUserAgent,
			SECTickersCachePath:      cfg.companyAnalysisSECTickersCacheFile,
			SECCompanyFactsCachePath: cfg.companyAnalysisSECCompanyFactsCacheFile,
		}),
	}
}

func authenticateAndRender(client *auth.KIClient, tokenCachePath string) error {
	token, err := client.EnsureAuthToken(context.Background())
	if err != nil {
		return fmt.Errorf("error ensuring auth token: %w", err)
	}

	renderAuthTokenSummary(tokenCachePath, client.AuthToken, token.TokenExpired)
	return nil
}

func runDomesticMarket(
	ctx context.Context,
	pbrCtx context.Context,
	cfg appConfig,
	deps appDeps,
	dates appDates,
) (domesticMarketState, error) {
	state := domesticMarketState{}

	respMarketTime, err := deps.domesticStock.MarketTime(ctx)
	mustAPIResult("market-time", respMarketTime, err, "output1")
	printMarketTimeSummary(respMarketTime)
	state.marketTime = respMarketTime
	state.businessDate = resolveBusinessDateFromMarketTime(respMarketTime, dates.today)

	quadRunWindow, err := quadwitching.EvaluateRunWindow(state.businessDate, cfg.quadWitchingLookaheadDays, cfg.quadWitchingGraceDays)
	if err != nil {
		log.Printf("quad witching schedule error: %v", err)
		quadRunWindow = quadwitching.RunWindow{
			BusinessDate:  state.businessDate,
			LookaheadDays: cfg.quadWitchingLookaheadDays,
			GraceDays:     cfg.quadWitchingGraceDays,
		}
	}
	state.quadRunWindow = quadRunWindow
	state.shouldRunQuadWitching = cfg.quadWitchingForce || quadRunWindow.ShouldRun
	printQuadWitchingGateSummary(quadRunWindow, cfg.quadWitchingForce, state.shouldRunQuadWitching)

	respKOSPI, err := deps.domesticStock.InquireIndexPrice(ctx, "0001")
	mustAPIResult("inquire-index-price (KOSPI 0001)", respKOSPI, err, "output")
	printIndexSummary("KOSPI", respKOSPI)
	state.kospi = respKOSPI

	respKOSDAQ, err := deps.domesticStock.InquireIndexPrice(ctx, "1001")
	mustAPIResult("inquire-index-price (KOSDAQ 1001)", respKOSDAQ, err, "output")
	printIndexSummary("KOSDAQ", respKOSDAQ)

	kospiActualPBR, err := deps.domesticStock.KOSPIActualPBR(pbrCtx, cfg.kospiProxyTargetCoverage, state.businessDate)
	if err != nil {
		return state, fmt.Errorf("KOSPI actual PBR error: %w", err)
	}
	printKOSPIActualPBRSummary(kospiActualPBR)

	vkospiCode, err := deps.domesticStock.ResolveVKOSPICode(ctx, nil)
	if err != nil {
		return state, fmt.Errorf("VKOSPI code resolve error: %w", err)
	}
	respVKOSPI, err := deps.domesticStock.InquireIndexPrice(ctx, vkospiCode)
	mustAPIResult("inquire-index-price (VKOSPI "+vkospiCode+")", respVKOSPI, err, "output")
	printIndexSummary("VKOSPI", respVKOSPI)

	respProgramTrade, err := deps.domesticStock.CompProgramTradeToday(ctx, "K")
	mustAPIResult("comp-program-trade-today (KOSPI)", respProgramTrade, err, "output")
	printProgramTradeSummary(respProgramTrade)
	state.programTrade = respProgramTrade

	respVI, err := deps.domesticStock.InquireVIStatus(ctx, state.businessDate)
	mustAPIResult("inquire-vi-status", respVI, err, "output")
	printVISummary(respVI)

	respFunds, err := deps.domesticStock.MarketFunds(ctx, "")
	mustAPIResult("mktfunds", respFunds, err, "output")
	printMarketFundsSummary(respFunds)

	return state, nil
}

func runQuadWitching(ctx context.Context, cfg appConfig, deps appDeps, market domesticMarketState) {
	if !market.shouldRunQuadWitching {
		log.Printf("skipping quad witching stats: outside configured window for %s", market.quadRunWindow.QuadDate)
		return
	}

	quadSnapshot := quadwitching.SnapshotExport{
		GeneratedAt:    time.Now().Format(time.RFC3339),
		BusinessDate:   market.businessDate,
		EndpointStates: map[string]quadwitching.EndpointSnapshot{},
		Notes: []string{
			"futures inquire-time-fuopccnl and inquire-member are experimental because no verified local sample existed in the repository",
		},
	}
	quadSnapshot.EndpointStates["market_time"] = quadwitching.NewEndpointSnapshot(market.marketTime, nil)
	quadSnapshot.EndpointStates["kospi_index_price"] = quadwitching.NewEndpointSnapshot(market.kospi, nil)
	quadSnapshot.EndpointStates["program_trade_today"] = quadwitching.NewEndpointSnapshot(market.programTrade, nil)

	quadWitchingFutures, err := deps.domesticFuture.ResolveNearMonthKOSPI200Futures(ctx, market.businessDate)
	if err != nil {
		log.Printf("quad witching futures code resolve error: %v", err)
		quadSnapshot.Notes = append(quadSnapshot.Notes, "futures resolver error: "+err.Error())
	} else {
		quadSnapshot.FuturesCode = quadWitchingFutures.Record.ShortCode
		quadSnapshot.FuturesName = quadWitchingFutures.Record.Name
		quadSnapshot.MasterCache = quadWitchingFutures.MasterCachePath
		quadSnapshot.MasterJSON = quadWitchingFutures.MasterJSONPath
		printQuadWitchingContractSummary(quadWitchingFutures)

		futuresCode := quadWitchingFutures.Record.ShortCode

		respFuturePrice, err := deps.domesticFuture.InquirePrice(ctx, cfg.quadWitchingFuturesMarketDivCode, futuresCode)
		quadSnapshot.EndpointStates["future_price"] = quadwitching.NewEndpointSnapshot(respFuturePrice, err)
		if optionalAPIResult("domestic-futureoption inquire-price ("+futuresCode+")", respFuturePrice, err, "output1") {
			printFuturePriceSummary(respFuturePrice)
		}

		respFutureBoardTop, err := deps.domesticFuture.DisplayBoardTop(ctx, cfg.quadWitchingFuturesMarketDivCode, futuresCode, "", "", "", "")
		quadSnapshot.EndpointStates["future_board_top"] = quadwitching.NewEndpointSnapshot(respFutureBoardTop, err)
		if optionalAPIResult("domestic-futureoption display-board-top ("+futuresCode+")", respFutureBoardTop, err, "output1") {
			printFutureBoardTopSummary(respFutureBoardTop)
		}

		respFutureBoard, err := deps.domesticFuture.DisplayBoardFutures(ctx, cfg.quadWitchingFuturesMarketDivCode, "", "")
		quadSnapshot.EndpointStates["future_board"] = quadwitching.NewEndpointSnapshot(respFutureBoard, err)
		if optionalAPIResult("domestic-futureoption display-board-futures", respFutureBoard, err, "output") {
			printFutureBoardSummary(respFutureBoard, futuresCode)
		}

		respFutureTimeChart, err := deps.domesticFuture.InquireTimeFuopChartPrice(
			ctx,
			cfg.quadWitchingFuturesMarketDivCode,
			futuresCode,
			cfg.quadWitchingFuturesHourClsCode,
			cfg.quadWitchingFuturesIncludePastData,
			cfg.quadWitchingFuturesIncludeFakeTick,
			market.businessDate,
			cfg.quadWitchingFuturesInputHour,
		)
		quadSnapshot.EndpointStates["future_time_chart"] = quadwitching.NewEndpointSnapshot(respFutureTimeChart, err)
		if optionalAPIResult("domestic-futureoption inquire-time-fuopchartprice ("+futuresCode+")", respFutureTimeChart, err, "output2") {
			printFutureTimeChartSummary(respFutureTimeChart)
		}

		respFutureExpected, err := deps.domesticFuture.ExpPriceTrend(ctx, futuresCode, cfg.quadWitchingFuturesMarketDivCode)
		quadSnapshot.EndpointStates["future_expected_price"] = quadwitching.NewEndpointSnapshot(respFutureExpected, err)
		if optionalAPIResult("domestic-futureoption exp-price-trend ("+futuresCode+")", respFutureExpected, err, "output2") {
			printFutureExpectedPriceSummary(respFutureExpected)
		}

		respFutureCCNL, err := deps.domesticFuture.InquireTimeFuopCCNL(ctx, cfg.quadWitchingFuturesMarketDivCode, futuresCode)
		quadSnapshot.EndpointStates["future_time_conclusion_experimental"] = quadwitching.NewEndpointSnapshot(respFutureCCNL, err)
		if optionalAPIResult("domestic-futureoption inquire-time-fuopccnl ("+futuresCode+")", respFutureCCNL, err, "output") {
			printFutureExecutionSummary(respFutureCCNL)
		}

		respFutureMember, err := deps.domesticFuture.InquireMember(ctx, cfg.quadWitchingFuturesMarketDivCode, futuresCode)
		quadSnapshot.EndpointStates["future_member_experimental"] = quadwitching.NewEndpointSnapshot(respFutureMember, err)
		if optionalAPIResult("domestic-futureoption inquire-member ("+futuresCode+")", respFutureMember, err, "output") {
			printFutureMemberSummary(respFutureMember)
		}
	}

	respQuadInvestor, err := deps.domesticStock.InquireInvestor(ctx, "J", cfg.quadWitchingInvestorSymbol)
	quadSnapshot.EndpointStates["kospi_investor"] = quadwitching.NewEndpointSnapshot(respQuadInvestor, err)
	if optionalAPIResult("domestic-stock inquire-investor", respQuadInvestor, err, "output") {
		printInvestorTrendSummary("Quad Witching KOSPI Investor Trend", respQuadInvestor)
	}

	respKOSPI200Investor, err := deps.domesticStock.InquireInvestor(ctx, "J", cfg.quadWitchingKOSPI200InvestorSymbol)
	quadSnapshot.EndpointStates["kospi200_investor"] = quadwitching.NewEndpointSnapshot(respKOSPI200Investor, err)
	if optionalAPIResult("domestic-stock inquire-investor (KOSPI200)", respKOSPI200Investor, err, "output") {
		printInvestorTrendSummary("Quad Witching KOSPI200 Investor Trend", respKOSPI200Investor)
	}

	respForeignTotal, err := deps.domesticStock.ForeignInstitutionTotal(
		ctx,
		cfg.quadWitchingForeignMarketDivCode,
		cfg.quadWitchingForeignScreenDivCode,
		cfg.quadWitchingForeignInputISCD,
		cfg.quadWitchingForeignDivClsCode,
		cfg.quadWitchingForeignRankSortCode,
		cfg.quadWitchingForeignEtcClsCode,
	)
	quadSnapshot.EndpointStates["foreign_institution_total"] = quadwitching.NewEndpointSnapshot(respForeignTotal, err)
	if optionalAPIResult("domestic-stock foreign-institution-total", respForeignTotal, err, "output") {
		printForeignInstitutionSummary(respForeignTotal)
	}

	respAskingPrice, err := deps.domesticStock.InquireAskingPriceExpCCN(ctx, "J", cfg.quadWitchingAuctionSymbol)
	quadSnapshot.EndpointStates["asking_price_expected"] = quadwitching.NewEndpointSnapshot(respAskingPrice, err)
	if optionalAPIResult("domestic-stock inquire-asking-price-exp-ccn", respAskingPrice, err, "output1") {
		printAskingPriceExpSummary(cfg.quadWitchingAuctionSymbol, respAskingPrice)
	}

	quadSnapshotPath := resolveQuadWitchingSnapshotPath(cfg.quadWitchingSnapshotJSONFile, market.businessDate, quadSnapshot.FuturesCode)
	if err := quadwitching.WriteSnapshot(quadSnapshotPath, quadSnapshot); err != nil {
		log.Printf("quad witching snapshot export error: %v", err)
		return
	}
	printQuadWitchingSnapshotSummary(quadSnapshotPath, quadSnapshot)
}

func runGlobalMarkets(ctx context.Context, cfg appConfig, deps appDeps, dates appDates) error {
	ewyExchangeCode, err := deps.overseasStock.ResolveEWYExchangeCode(ctx)
	if err != nil {
		return fmt.Errorf("EWY exchange code resolve error: %w", err)
	}
	respEWY, err := deps.overseasStock.Price(ctx, ewyExchangeCode, cfg.ewySymbol)
	mustAPIResult("overseas-stock price ("+ewyExchangeCode+" "+cfg.ewySymbol+")", respEWY, err, "output")
	printOverseasStockSummary("EWY", respEWY)

	respExchangeRate, err := deps.overseasStock.InquireDailyChartPrice(
		ctx,
		cfg.exchangeRateMarketDivCode,
		cfg.exchangeRateSymbol,
		dates.exchangeRateFromDate,
		dates.today,
		"D",
	)
	mustAPIResult(
		"exchange-rate daily-chart ("+cfg.exchangeRateMarketDivCode+" "+cfg.exchangeRateSymbol+")",
		respExchangeRate,
		err,
		"output1",
	)
	printExchangeRateSummary(cfg.exchangeRateMarketDivCode, cfg.exchangeRateSymbol, respExchangeRate)

	for _, instrument := range []commodityfuture.Instrument{
		configuredCommodityInstrument("Copper", cfg.copperFutureTicker, cfg.copperFutureProductCode, cfg.copperFutureCMEProductID),
		configuredCommodityInstrument("Gold", cfg.goldFutureTicker, cfg.goldFutureProductCode, cfg.goldFutureCMEProductID),
		configuredCommodityInstrument("Silver", cfg.silverFutureTicker, cfg.silverFutureProductCode, cfg.silverFutureCMEProductID),
	} {
		quote, err := runConfiguredCommodityFuture(ctx, deps.commodityFuture, instrument)
		if err != nil {
			log.Printf("%s future quote error: %v", instrument.Name, err)
			continue
		}
		printCommodityFutureSummary(joinNonEmpty(" ", instrument.Name, instrument.Symbol), quote)
	}

	shippingQuotes, err := deps.shippingIndex.Quotes(ctx, cfg.shippingIndexSymbols)
	if err != nil {
		log.Printf("shipping index error: %v", err)
	}
	printShippingIndexSummary(shippingQuotes)
	return nil
}

func runConfiguredCommodityFuture(
	ctx context.Context,
	svc *commodityfuture.Service,
	instrument commodityfuture.Instrument,
) (*commodityfuture.Quote, error) {
	return svc.Quote(ctx, instrument)
}

func runCompanyAnalysis(ctx context.Context, cfg appConfig, deps appDeps, businessDate string) {
	result, err := deps.companyAnalysisSvc.Analyze(ctx, cfg.companyAnalysisSymbol, companyanalysis.AnalysisOptions{
		BenchmarkSymbol:  cfg.companyAnalysisBenchmarkSymbol,
		ForecastYears:    cfg.companyAnalysisForecastYears,
		TerminalGrowth:   cfg.companyAnalysisTerminalGrowth,
		BetaLookbackDays: cfg.companyAnalysisBetaLookbackDays,
		RiskFreeRate:     getOptionalFloat("COMPANY_ANALYSIS_RISK_FREE_RATE"),
		Beta:             getOptionalFloat("COMPANY_ANALYSIS_BETA"),
		MarketPremium:    getOptionalFloat("COMPANY_ANALYSIS_MARKET_PREMIUM"),
		CostOfDebt:       getOptionalFloat("COMPANY_ANALYSIS_COST_OF_DEBT"),
		NetDebt:          getOptionalFloat("COMPANY_ANALYSIS_NET_DEBT"),
	})
	if err != nil {
		log.Printf("company analysis error: %v", err)
		return
	}

	printCompanyAnalysisSummary(result)

	var companyReverseDCF *dcf.ReverseDCFResult
	if result.Quote.Price > 0 {
		companyReverseDCF, err = dcf.ReverseDCF(
			result.Financial,
			result.Market,
			result.Assumptions,
			result.Projection,
			result.Quote.Price,
			dcf.ReverseDCFConfig{},
		)
		if err != nil {
			log.Printf("company reverse DCF error: %v", err)
		} else {
			printCompanyReverseDCFSummary(result, companyReverseDCF)
		}
	}

	companyMonteCarloConfig := newMonteCarloConfig(cfg)
	companyMonteCarlo, companyMonteCarloErr := dcf.MonteCarlo(
		result.Financial,
		result.Market,
		result.Assumptions,
		result.Projection,
		companyMonteCarloConfig,
	)
	companyAnalysisJSONPath := resolveCompanyAnalysisJSONPath(cfg.companyAnalysisJSONFile, businessDate, cfg.companyAnalysisSymbol)
	if companyMonteCarloErr != nil {
		log.Printf("company monte carlo DCF error: %v", companyMonteCarloErr)
	} else {
		printCompanyMonteCarloSummary(companyMonteCarlo, companyAnalysisJSONPath)
	}
	if err := companyanalysis.WriteExport(companyAnalysisJSONPath, companyanalysis.Export{
		GeneratedAt:   time.Now().Format(time.RFC3339),
		BusinessDate:  businessDate,
		Symbol:        cfg.companyAnalysisSymbol,
		Result:        result,
		ReverseDCF:    companyReverseDCF,
		MonteCarloCfg: companyMonteCarloConfig,
		MonteCarlo:    companyMonteCarlo,
	}); err != nil {
		log.Printf("company analysis JSON export error: %v", err)
	}
}

func runDomesticValuation(
	ctx context.Context,
	cfg appConfig,
	deps appDeps,
	dates appDates,
	businessDate string,
) error {
	rsiResult, err := deps.domesticStock.RSIFromDailyChart(ctx, cfg.targetSymbol, 14, dates.fromDate, dates.today)
	if err != nil {
		return fmt.Errorf("RSI calculation error: %w", err)
	}
	fmt.Printf("\n--- RSI Result ---\n")
	fmt.Printf("Symbol: %s\n", rsiResult.Symbol)
	fmt.Printf("Period: %d\n", rsiResult.Period)
	fmt.Printf("Sample Size: %d\n", rsiResult.SampleSize)
	fmt.Printf("RSI: %.2f\n", rsiResult.Last)
	fmt.Printf("Signal: %s\n", rsiResult.Signal)
	fmt.Printf("------------------\n")

	dcfReadiness, err := deps.domesticStock.DCFReadiness(ctx, cfg.targetSymbol, "0")
	if err != nil {
		log.Printf("DCF readiness error: %v", err)
	} else {
		printDCFReadinessSummary(dcfReadiness)
	}

	dcfValuation, err := deps.domesticStock.DCFValuation(ctx, cfg.targetSymbol, domesticstock.DCFValuationOptions{})
	if err != nil {
		log.Printf("DCF valuation error: %v", err)
	} else {
		printDCFValuationSummary(dcfValuation)
	}

	var reverseDCFResult *dcf.ReverseDCFResult
	if dcfValuation != nil {
		if dcfValuation.CurrentPrice <= 0 {
			log.Printf("reverse DCF error: current price missing")
		} else {
			reverseDCFResult, err = dcf.ReverseDCF(
				dcfValuation.Financial,
				dcfValuation.Market,
				dcfValuation.Assumptions,
				dcfValuation.Projection,
				dcfValuation.CurrentPrice,
				dcf.ReverseDCFConfig{},
			)
			if err != nil {
				log.Printf("reverse DCF error: %v", err)
			} else {
				printReverseDCFSummary(reverseDCFResult)
			}
		}
	}

	if dcfValuation != nil {
		monteCarloConfig := newMonteCarloConfig(cfg)
		monteCarloResult, monteCarloErr := dcf.MonteCarlo(
			dcfValuation.Financial,
			dcfValuation.Market,
			dcfValuation.Assumptions,
			dcfValuation.Projection,
			monteCarloConfig,
		)
		if monteCarloErr != nil {
			log.Printf("monte carlo DCF error: %v", monteCarloErr)
		} else {
			monteCarloJSONPath := resolveMonteCarloJSONPath(cfg.dcfMonteCarloJSONFile, businessDate, cfg.targetSymbol)
			if err := dcf.WriteMonteCarloExport(monteCarloJSONPath, dcf.MonteCarloExport{
				GeneratedAt:   time.Now().Format(time.RFC3339),
				BusinessDate:  businessDate,
				Symbol:        cfg.targetSymbol,
				CurrentPrice:  dcfValuation.CurrentPrice,
				Financial:     dcfValuation.Financial,
				Market:        dcfValuation.Market,
				Assumptions:   dcfValuation.Assumptions,
				Projection:    dcfValuation.Projection,
				Valuation:     dcfValuation.Valuation,
				ReverseDCF:    reverseDCFResult,
				MonteCarloCfg: monteCarloConfig,
				MonteCarlo:    monteCarloResult,
			}); err != nil {
				log.Printf("monte carlo JSON export error: %v", err)
			}
			printMonteCarloSummary(monteCarloResult, monteCarloJSONPath)
		}
	}

	return nil
}

func newMonteCarloConfig(cfg appConfig) dcf.MonteCarloConfig {
	return dcf.MonteCarloConfig{
		Iterations:           cfg.dcfMonteCarloIterations,
		Workers:              cfg.dcfMonteCarloWorkers,
		RevenueGrowthStdDev:  cfg.dcfMonteCarloGrowthStdDev,
		WACCStdDev:           cfg.dcfMonteCarloWACCStdDev,
		TerminalGrowthStdDev: cfg.dcfMonteCarloTerminalStdDev,
	}
}
