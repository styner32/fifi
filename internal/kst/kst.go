package kst

import "time"

// Location is the KST (Asia/Seoul) timezone location. Fallbacks to FixedZone if loading fails.
var Location = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		return time.FixedZone("KST", 9*3600)
	}
	return loc
}()

// Now returns the current time in KST.
func Now() time.Time {
	return time.Now().In(Location)
}
