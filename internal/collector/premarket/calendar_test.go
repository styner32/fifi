package premarket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const calendarFixture = "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:cpi\r\nSUMMARY:Consumer Price In\r\n dex\\, August\r\nDTSTART;TZID=America/New_York:20260914T083000\r\nEND:VEVENT\r\nBEGIN:VEVENT\r\nUID:winter\r\nSUMMARY:Winter release\r\nDTSTART;TZID=America/New_York:20260114T083000\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"

func TestCalendarFoldingAndDST(t *testing.T) {
	events, err := parseCalendarICS(calendarFixture, blsCalendarURL)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Title != "Consumer Price Index, August" {
		t.Fatal(events)
	}
	if events[0].ScheduledAt.In(kst).Hour() != 21 || events[1].ScheduledAt.In(kst).Hour() != 22 {
		t.Fatal("Eastern daylight savings conversion failed")
	}
	for _, body := range []string{"<html>blocked</html>", "BEGIN:VCALENDAR\nBEGIN:VEVENT\nSUMMARY:partial", strings.Replace(calendarFixture, "America/New_York", "unknown/timezone", 1)} {
		if _, err := parseCalendarICS(body, blsCalendarURL); err == nil {
			t.Fatal("invalid calendar accepted")
		}
	}
}
func TestFOMCDateOnlyMeeting(t *testing.T) {
	body := `<div class="panel"><div class="panel-heading">2026 FOMC Meetings</div><div class="fomc-meeting"><div class="fomc-meeting__month">September</div><div class="fomc-meeting__date">15-16*</div></div></div>`
	e, err := parseFOMC(body, 2026)
	if err != nil || len(e) != 1 {
		t.Fatalf("%+v %v", e, err)
	}
	if e[0].ScheduledAt != nil || e[0].SourceDate != "2026-09-15" || e[0].SourceEndDate != "2026-09-16" {
		t.Fatal("invented statement time or wrong local meeting date")
	}
}

type fixtureCalendar struct{ events []CalendarEvent }

func (f fixtureCalendar) FetchCalendar(context.Context, int) ([]CalendarEvent, []CalendarSourceStatus) {
	return f.events, []CalendarSourceStatus{{Name: "BEA", URL: beaCalendarURL, Status: "VALID"}, {Name: "BLS", URL: blsCalendarURL, Status: "UNAVAILABLE", Reason: "HTTP 403"}}
}
func TestCalendarWindowAndPartialFailure(t *testing.T) {
	target := time.Date(2026, 9, 14, 0, 0, 0, 0, kst)
	a := target.Add(time.Hour)
	b := target.AddDate(0, 0, 7)
	events := []CalendarEvent{{ID: "a", Title: "Release", ScheduledAt: &a, SourceURL: beaCalendarURL, Confirmation: "CONFIRMED"}, {ID: "b", Title: "Outside", ScheduledAt: &b, SourceURL: beaCalendarURL, Confirmation: "CONFIRMED"}}
	r := collectCalendar(context.Background(), fixtureCalendar{events}, target, target.Add(8*time.Hour), target.Add(8*time.Hour))
	if r.Status != "PARTIAL" || len(r.Events) != 1 || r.Events[0].State != "SCHEDULED_TIME_PASSED_RELEASE_UNVERIFIED" || len(r.Uncovered) == 0 {
		t.Fatalf("%+v", r)
	}
}

type calendarTestTransport func(http.ResponseWriter, *http.Request)

func (f calendarTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	f(w, req)
	return w.Result(), nil
}
func TestOfficialCalendarSourceFailureDoesNotDiscardOthers(t *testing.T) {
	handler := calendarTestTransport(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "fomc") {
			w.Write([]byte(`<div class="panel"><div class="panel-heading">2026 FOMC Meetings</div><div class="fomc-meeting"><div class="fomc-meeting__month">September</div><div class="fomc-meeting__date">15-16</div></div></div>`))
		} else if strings.Contains(r.URL.Path, "bls") {
			w.WriteHeader(http.StatusForbidden)
		} else {
			w.Write([]byte(calendarFixture))
		}
	})
	c := OfficialCalendar{Client: &http.Client{Transport: handler}}
	events, status := c.FetchCalendar(context.Background(), 2026)
	if len(events) != 3 || len(status) != 3 || status[1].Status != "UNAVAILABLE" || status[2].Status != "VALID" {
		t.Fatalf("%+v %+v", events, status)
	}
}
