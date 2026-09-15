package premarket

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const fedCalendarURL = "https://www.federalreserve.gov/monetarypolicy/fomccalendars.htm"
const blsCalendarURL = "https://www.bls.gov/schedule/news_release/bls.ics"
const beaCalendarURL = "https://www.bea.gov/news/schedule/ics/online-calendar-subscription.ics"

type CalendarEvent struct {
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	ScheduledAt   *time.Time `json:"scheduled_at"`
	SourceDate    string     `json:"source_date"`
	SourceEndDate string     `json:"source_end_date,omitempty"`
	Timezone      string     `json:"timezone"`
	SourceURL     string     `json:"source_url"`
	Confirmation  string     `json:"confirmation"`
	State         string     `json:"state"`
	FetchedAt     time.Time  `json:"fetched_at"`
}
type CalendarSourceStatus struct {
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Status    string    `json:"status"`
	Reason    string    `json:"reason,omitempty"`
	FetchedAt time.Time `json:"fetched_at"`
}
type CalendarReport struct {
	Status      string                 `json:"status"`
	WindowStart string                 `json:"window_start"`
	WindowEnd   string                 `json:"window_end_exclusive"`
	Events      []CalendarEvent        `json:"events"`
	Sources     []CalendarSourceStatus `json:"sources"`
	Uncovered   []string               `json:"uncovered"`
}
type EventCalendar interface {
	FetchCalendar(context.Context, int) ([]CalendarEvent, []CalendarSourceStatus)
}

func emptyCalendar() CalendarReport {
	return CalendarReport{Status: "UNAVAILABLE", Events: []CalendarEvent{}, Sources: []CalendarSourceStatus{}, Uncovered: []string{"한국은행·국내 경제지표", "기업 실적 일정", "파생 만기·지수 정기변경"}}
}

type OfficialCalendar struct {
	Client    *http.Client
	UserAgent string
}

func (c *OfficialCalendar) FetchCalendar(ctx context.Context, year int) ([]CalendarEvent, []CalendarSourceStatus) {
	events := []CalendarEvent{}
	statuses := []CalendarSourceStatus{}
	for _, s := range []struct{ name, url string }{{"FOMC", fedCalendarURL}, {"BLS", blsCalendarURL}, {"BEA", beaCalendarURL}} {
		status := CalendarSourceStatus{Name: s.name, URL: s.url, Status: "UNAVAILABLE"}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
		if err != nil {
			status.Reason = "request creation failed"
			statuses = append(statuses, status)
			continue
		}
		ua := c.UserAgent
		if ua == "" {
			ua = "Mozilla/5.0"
		}
		req.Header.Set("User-Agent", ua)
		client := c.Client
		if client == nil {
			client = &http.Client{Timeout: 15 * time.Second}
		}
		resp, err := client.Do(req)
		if err != nil {
			status.Reason = "official calendar network request failed"
			statuses = append(statuses, status)
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK {
			status.Reason = fmt.Sprintf("official calendar HTTP %d or read error", resp.StatusCode)
			statuses = append(statuses, status)
			continue
		}
		var parsed []CalendarEvent
		if s.name == "FOMC" {
			parsed, err = parseFOMC(string(body), year)
		} else {
			parsed, err = parseCalendarICS(string(body), s.url)
		}
		if err != nil {
			status.Reason = err.Error()
		} else {
			status.Status = "VALID"
			events = append(events, parsed...)
		}
		statuses = append(statuses, status)
	}
	return events, statuses
}

func collectCalendar(ctx context.Context, client EventCalendar, target, cutoff, now time.Time) CalendarReport {
	r := emptyCalendar()
	end := target.AddDate(0, 0, 7)
	r.WindowStart = target.Format("2006-01-02")
	r.WindowEnd = end.Format("2006-01-02")
	events, statuses := client.FetchCalendar(ctx, target.Year())
	r.Sources = statuses
	successes := 0
	for i := range r.Sources {
		r.Sources[i].FetchedAt = now
		if r.Sources[i].Status == "VALID" {
			successes++
		}
	}
	if successes > 0 {
		r.Status = "PARTIAL"
	} // Connected feeds do not cover the whole market event universe.
	seen := map[string]bool{}
	for _, e := range events {
		if e.ID == "" || e.Title == "" || e.SourceURL == "" || seen[e.ID] || e.Confirmation == "CANCELLED" {
			continue
		}
		if e.ScheduledAt != nil {
			t := e.ScheduledAt.In(kst)
			if t.Before(target) || !t.Before(end) {
				continue
			}
			e.ScheduledAt = &t
			e.State = "SCHEDULED"
			if !t.After(cutoff) {
				e.State = "SCHEDULED_TIME_PASSED_RELEASE_UNVERIFIED"
			}
		} else {
			// A date-only US meeting is not a midnight KST announcement. Preserve its
			// local dates and include potential overlap with the KST window.
			loc, err := time.LoadLocation(e.Timezone)
			if err != nil {
				continue
			}
			start, err := time.ParseInLocation("2006-01-02", e.SourceDate, loc)
			if err != nil {
				continue
			}
			last := start
			if e.SourceEndDate != "" {
				last, err = time.ParseInLocation("2006-01-02", e.SourceEndDate, loc)
				if err != nil {
					continue
				}
			}
			if !start.Before(end) || !last.AddDate(0, 0, 1).After(target) {
				continue
			}
			e.State = "DATE_CONFIRMED_TIME_UNAVAILABLE"
		}
		seen[e.ID] = true
		e.FetchedAt = now
		r.Events = append(r.Events, e)
	}
	sort.SliceStable(r.Events, func(i, j int) bool { return eventSortKey(r.Events[i]) < eventSortKey(r.Events[j]) })
	return r
}
func eventSortKey(e CalendarEvent) string {
	if e.ScheduledAt != nil {
		return e.ScheduledAt.In(kst).Format(time.RFC3339) + e.Title
	}
	return e.SourceDate + e.Title
}

func parseCalendarICS(body, source string) ([]CalendarEvent, error) {
	// RFC 5545 line folding occurs inside titles and identifiers.
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\n ", "")
	body = strings.ReplaceAll(body, "\n\t", "")
	if !strings.Contains(body, "BEGIN:VCALENDAR") || !strings.Contains(body, "END:VCALENDAR") {
		return nil, fmt.Errorf("calendar response is not iCalendar")
	}
	out := []CalendarEvent{}
	props := map[string]string{}
	params := map[string]string{}
	inside := false
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "BEGIN:VEVENT" {
			inside = true
			props = map[string]string{}
			params = map[string]string{}
			continue
		}
		if line == "END:VEVENT" {
			if !inside {
				return nil, fmt.Errorf("malformed calendar event")
			}
			inside = false
			if props["STATUS"] == "CANCELLED" {
				continue
			}
			if props["RRULE"] != "" || props["RECURRENCE-ID"] != "" {
				return nil, fmt.Errorf("unsupported recurring calendar event")
			}
			title := unescapeICS(props["SUMMARY"])
			value := props["DTSTART"]
			if title == "" || value == "" {
				return nil, fmt.Errorf("calendar event missing title or date")
			}
			e := CalendarEvent{ID: source + "#" + props["UID"], Title: title, SourceURL: source, Confirmation: "CONFIRMED", Timezone: "America/New_York"}
			if props["STATUS"] == "TENTATIVE" {
				e.Confirmation = "TENTATIVE"
			}
			if props["UID"] == "" {
				e.ID = source + "#" + value + "#" + title
			}
			if strings.Contains(params["DTSTART"], "VALUE=DATE") && !strings.Contains(params["DTSTART"], "DATE-TIME") {
				d, err := time.Parse("20060102", value)
				if err != nil {
					return nil, fmt.Errorf("invalid all-day calendar date")
				}
				e.SourceDate = d.Format("2006-01-02")
			} else {
				loc, err := time.LoadLocation("America/New_York")
				if err != nil {
					return nil, err
				}
				for _, p := range strings.Split(params["DTSTART"], ";") {
					if strings.HasPrefix(p, "TZID=") {
						loc, err = time.LoadLocation(strings.TrimPrefix(p, "TZID="))
						if err != nil {
							return nil, fmt.Errorf("unsupported calendar timezone")
						}
					}
				}
				var t time.Time
				if strings.HasSuffix(value, "Z") {
					t, err = time.Parse("20060102T150405Z", value)
				} else {
					t, err = time.ParseInLocation("20060102T150405", value, loc)
				}
				if err != nil {
					return nil, fmt.Errorf("invalid calendar timestamp")
				}
				e.ScheduledAt = &t
				e.SourceDate = t.In(loc).Format("2006-01-02")
				e.Timezone = loc.String()
			}
			out = append(out, e)
			continue
		}
		if inside {
			kv := strings.SplitN(line, ":", 2)
			if len(kv) != 2 {
				continue
			}
			key := strings.SplitN(kv[0], ";", 2)
			props[key[0]] = kv[1]
			if len(key) > 1 {
				params[key[0]] = key[1]
			}
		}
	}
	if inside || len(out) == 0 {
		return nil, fmt.Errorf("empty or incomplete official calendar")
	}
	return out, nil
}
func unescapeICS(s string) string {
	return strings.NewReplacer("\\n", " ", "\\N", " ", "\\,", ",", "\\;", ";", "\\\\", "\\").Replace(s)
}

func parseFOMC(body string, year int) ([]CalendarEvent, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	out := []CalendarEvent{}
	var parseErr error
	doc.Find(".panel").Each(func(_ int, panel *goquery.Selection) {
		heading := strings.TrimSpace(panel.Find(".panel-heading").First().Text())
		if !strings.HasPrefix(heading, fmt.Sprintf("%d FOMC Meetings", year)) && !strings.HasPrefix(heading, fmt.Sprintf("%d FOMC Meetings", year+1)) {
			return
		}
		y, err := strconv.Atoi(heading[:4])
		if err != nil {
			return
		}
		panel.Find(".fomc-meeting").Each(func(_ int, row *goquery.Selection) {
			month := strings.TrimSpace(row.Find(".fomc-meeting__month").Text())
			days := strings.TrimSpace(row.Find(".fomc-meeting__date").Text())
			// Unscheduled notation votes are not regular scheduled meetings.
			if strings.Contains(days, "(") {
				return
			}
			months := strings.Split(month, "/")
			nums := regexp.MustCompile(`\d+`).FindAllString(days, -1)
			if len(nums) == 0 || len(nums) > 2 {
				parseErr = fmt.Errorf("unsupported FOMC meeting date")
				return
			}
			first, err := time.Parse("2006 January 2", fmt.Sprintf("%d %s %s", y, strings.TrimSpace(months[0]), nums[0]))
			if err != nil {
				parseErr = fmt.Errorf("invalid FOMC date")
				return
			}
			last, err := time.Parse("2006 January 2", fmt.Sprintf("%d %s %s", y, strings.TrimSpace(months[len(months)-1]), nums[len(nums)-1]))
			if err != nil || last.Before(first) {
				parseErr = fmt.Errorf("invalid FOMC end date")
				return
			}
			title := "FOMC 회의"
			if strings.Contains(days, "*") {
				title += " (경제전망 발표 회의)"
			}
			confirmation := "CONFIRMED"
			if y > year {
				confirmation = "TENTATIVE"
			}
			out = append(out, CalendarEvent{ID: "FOMC-" + first.Format("20060102"), Title: title, SourceDate: first.Format("2006-01-02"), SourceEndDate: last.Format("2006-01-02"), Timezone: "America/New_York", SourceURL: fedCalendarURL, Confirmation: confirmation})
		})
	})
	if parseErr != nil {
		return nil, parseErr
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("FOMC calendar has no matching year")
	}
	return out, nil
}
