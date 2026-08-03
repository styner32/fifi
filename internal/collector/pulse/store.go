package pulse

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// pulseFilePath는 날짜별 JSONL 파일 경로를 반환합니다.
func pulseFilePath(dir, date string) string {
	return filepath.Join(dir, fmt.Sprintf("pulse_%s.jsonl", date))
}

// pulseMDPath는 날짜별 Markdown 파일 경로를 반환합니다.
func pulseMDPath(dir, date string) string {
	return filepath.Join(dir, fmt.Sprintf("pulse_%s.md", date))
}

// AppendRecord는 PulseRecord를 JSONL에 한 줄 추가합니다.
func AppendRecord(dir, date string, rec PulseRecord) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	path := pulseFilePath(dir, date)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	line, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal record: %w", err)
	}
	_, err = f.Write(append(line, '\n'))
	return err
}

// LoadRecords는 당일 JSONL에서 모든 레코드를 읽습니다.
// 파일이 없으면 nil, nil 반환.
func LoadRecords(dir, date string) ([]PulseRecord, error) {
	path := pulseFilePath(dir, date)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var records []PulseRecord
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec PulseRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue // 손상된 줄 무시
		}
		records = append(records, rec)
	}
	return records, sc.Err()
}

// LoadNearest는 records에서 (now - delta) 시각에 가장 가까우면서
// 그 이전(at-or-before)인 레코드를 반환합니다. 없으면 nil.
func LoadNearest(records []PulseRecord, target time.Time) *PulseRecord {
	var best *PulseRecord
	for i := range records {
		rec := &records[i]
		if !rec.TS.After(target) {
			if best == nil || rec.TS.After(best.TS) {
				best = rec
			}
		}
	}
	return best
}
