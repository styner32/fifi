package premarket

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

type DailyRecord struct {
	Date                 string             `json:"date"`
	KOSPIPrice           float64            `json:"kospi_price"`
	KOSDAQPrice          float64            `json:"kosdaq_price"`
	CreditLoanBalanceEok float64            `json:"credit_loan_balance_eok"`
	MarginReceivableEok  float64            `json:"margin_receivable_eok"`
	ForcedSellAmountEok  float64            `json:"forced_sell_amount_eok"`
	CustomerDepositEok   float64            `json:"customer_deposit_eok"`
	VKOSPI               float64            `json:"vkospi"`
	SKHYADRClose         float64            `json:"skhy_adr_close"`
	USSemiComposite      float64            `json:"us_semi_composite"`
	EchoEvents           []EchoEvent        `json:"echo_events,omitempty"`
	Extra                map[string]float64 `json:"extra,omitempty"`
}

type StoreData struct {
	Records []DailyRecord `json:"records"`
}

type Store struct {
	mu       sync.RWMutex
	filePath string
	Data     StoreData
}

func NewStore(dir string) *Store {
	if dir == "" {
		dir = ".cache/premarket"
	}
	_ = os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, "store.json")
	s := &Store{filePath: path, Data: StoreData{Records: []DailyRecord{}}}
	s.load()
	return s
}

func (s *Store) load() {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.Open(s.filePath)
	if err != nil {
		return
	}
	defer f.Close()
	_ = json.NewDecoder(f).Decode(&s.Data)
}

func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sort.Slice(s.Data.Records, func(i, j int) bool {
		return s.Data.Records[i].Date < s.Data.Records[j].Date
	})

	// Max 300 records
	if len(s.Data.Records) > 300 {
		s.Data.Records = s.Data.Records[len(s.Data.Records)-300:]
	}

	_ = os.MkdirAll(filepath.Dir(s.filePath), 0o755)
	f, err := os.Create(s.filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(&s.Data)
}

func (s *Store) UpsertRecord(rec DailyRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()

	found := false
	for i, r := range s.Data.Records {
		if r.Date == rec.Date {
			// Merge fields
			if rec.KOSPIPrice > 0 { s.Data.Records[i].KOSPIPrice = rec.KOSPIPrice }
			if rec.KOSDAQPrice > 0 { s.Data.Records[i].KOSDAQPrice = rec.KOSDAQPrice }
			if rec.CreditLoanBalanceEok > 0 { s.Data.Records[i].CreditLoanBalanceEok = rec.CreditLoanBalanceEok }
			if rec.MarginReceivableEok > 0 { s.Data.Records[i].MarginReceivableEok = rec.MarginReceivableEok }
			if rec.ForcedSellAmountEok > 0 { s.Data.Records[i].ForcedSellAmountEok = rec.ForcedSellAmountEok }
			if rec.CustomerDepositEok > 0 { s.Data.Records[i].CustomerDepositEok = rec.CustomerDepositEok }
			if rec.VKOSPI > 0 { s.Data.Records[i].VKOSPI = rec.VKOSPI }
			if rec.SKHYADRClose > 0 { s.Data.Records[i].SKHYADRClose = rec.SKHYADRClose }
			if rec.USSemiComposite != 0 { s.Data.Records[i].USSemiComposite = rec.USSemiComposite }
			if len(rec.EchoEvents) > 0 { s.Data.Records[i].EchoEvents = rec.EchoEvents }
			found = true
			break
		}
	}
	if !found {
		s.Data.Records = append(s.Data.Records, rec)
	}
}

func (s *Store) GetLatestVKOSPICcloses(limit int) []float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []float64
	for i := len(s.Data.Records) - 1; i >= 0; i-- {
		if s.Data.Records[i].VKOSPI > 0 {
			out = append(out, s.Data.Records[i].VKOSPI)
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}

func (s *Store) GetLatestCreditBalances(limit int) []float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []float64
	for i := len(s.Data.Records) - 1; i >= 0; i-- {
		if s.Data.Records[i].CreditLoanBalanceEok > 0 {
			out = append(out, s.Data.Records[i].CreditLoanBalanceEok)
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}
