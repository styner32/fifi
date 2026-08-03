package quadwitching

import (
	"fmt"
	"strings"
	"time"
)

type RunWindow struct {
	BusinessDate  string `json:"business_date"`
	QuadDate      string `json:"quad_date"`
	WindowStart   string `json:"window_start"`
	WindowEnd     string `json:"window_end"`
	LookaheadDays int    `json:"lookahead_days"`
	GraceDays     int    `json:"grace_days"`
	DaysUntil     int    `json:"days_until"`
	ShouldRun     bool   `json:"should_run"`
}

func EvaluateRunWindow(businessDate string, lookaheadDays int, graceDays int) (RunWindow, error) {
	businessDate = strings.TrimSpace(businessDate)
	if businessDate == "" {
		return RunWindow{}, fmt.Errorf("businessDate is required")
	}
	if lookaheadDays < 0 {
		lookaheadDays = 0
	}
	if graceDays < 0 {
		graceDays = 0
	}

	date, err := time.ParseInLocation("20060102", businessDate, time.Local)
	if err != nil {
		return RunWindow{}, fmt.Errorf("invalid businessDate %q: %w", businessDate, err)
	}

	candidates := quadDatesAround(date.Year())
	for _, quadDate := range candidates {
		window := buildRunWindow(date, quadDate, lookaheadDays, graceDays)
		if window.ShouldRun {
			return window, nil
		}
	}

	for _, quadDate := range candidates {
		if quadDate.Before(date) {
			continue
		}
		return buildRunWindow(date, quadDate, lookaheadDays, graceDays), nil
	}

	return buildRunWindow(date, secondThursday(date.Year()+1, time.March), lookaheadDays, graceDays), nil
}

func buildRunWindow(date time.Time, quadDate time.Time, lookaheadDays int, graceDays int) RunWindow {
	windowStart := quadDate.AddDate(0, 0, -lookaheadDays)
	windowEnd := quadDate.AddDate(0, 0, graceDays)

	return RunWindow{
		BusinessDate:  date.Format("20060102"),
		QuadDate:      quadDate.Format("20060102"),
		WindowStart:   windowStart.Format("20060102"),
		WindowEnd:     windowEnd.Format("20060102"),
		LookaheadDays: lookaheadDays,
		GraceDays:     graceDays,
		DaysUntil:     calendarDaysBetween(date, quadDate),
		ShouldRun:     !date.Before(windowStart) && !date.After(windowEnd),
	}
}

func quadDatesAround(year int) []time.Time {
	years := []int{year - 1, year, year + 1}
	months := []time.Month{time.March, time.June, time.September, time.December}

	dates := make([]time.Time, 0, len(years)*len(months))
	for _, candidateYear := range years {
		for _, month := range months {
			dates = append(dates, secondThursday(candidateYear, month))
		}
	}
	return dates
}

func secondThursday(year int, month time.Month) time.Time {
	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	offset := (int(time.Thursday) - int(firstDay.Weekday()) + 7) % 7
	firstThursdayDay := 1 + offset
	return time.Date(year, month, firstThursdayDay+7, 0, 0, 0, 0, time.Local)
}

func calendarDaysBetween(from time.Time, to time.Time) int {
	return int(to.Sub(from).Hours() / 24)
}
