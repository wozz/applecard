package applecard

import (
	"bytes"
	"encoding/csv"
	"regexp"
	"strings"

	"github.com/ledongthuc/pdf"
)

func readPDFLines(filename string) ([]string, error) {
	f, r, err := pdf.Open(filename)
	defer f.Close()
	if err != nil {
		return nil, err
	}
	b, err := r.GetPlainText()
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.ReadFrom(b)
	return strings.Split(buf.String(), "\n"), nil
}

type transaction struct {
	date              string
	location          string
	transactionAmt    string
	cashBackPct       string
	cashBackAmt       string
	promoCashAmt      string
	promoCashPct      string
	cashAdjustmentAmt string
	cashAdjustmentPct string
}

func (_ transaction) header() []string {
	return []string{
		"date",
		"location",
		"amount",
		"cash_back_pct",
		"cash_back_amt",
		"promo_cash_amt",
		"promo_cash_pct",
		"cash_adj_amt",
		"cash_adj_pct",
	}
}

func (t transaction) record() []string {
	return []string{
		t.date,
		t.location,
		t.transactionAmt,
		t.cashBackPct,
		t.cashBackAmt,
		t.promoCashAmt,
		t.promoCashPct,
		t.cashAdjustmentAmt,
		t.cashAdjustmentPct,
	}
}

type state int

const (
	preamble state = iota
	txHeader
	txList
	cashAdj
	promo
	pageBreak
	end
)

var pageBreakSentinel = regexp.MustCompile("^Page [0-9]+ /[0-9]+$")
var percentFormat = regexp.MustCompile("^[-]?[0-9]+%$")

func parseTransactions(lines []string) ([]transaction, error) {
	var txs []transaction
	curState := preamble
	stateChangeNum := 0
	var tx transaction
	for i, l := range lines {
		if (curState == preamble || curState == pageBreak) && l == "Transactions" {
			curState = txHeader
			stateChangeNum = i
			continue
		}
		if curState == txHeader && i == stateChangeNum+4 {
			curState = txList
			stateChangeNum = i
			continue
		}
		if curState == promo && i <= stateChangeNum+3 {
			if i == stateChangeNum+1 {
				tx.promoCashPct = l
			} else if i == stateChangeNum+2 {
				tx.promoCashAmt = l
			} else {
				curState = txList
				stateChangeNum = i
				txs[len(txs)-1] = tx
				tx = transaction{}
			}
			continue
		}
		if curState == cashAdj && i <= stateChangeNum+2 {
			if i == stateChangeNum+1 {
				tx.cashAdjustmentPct = l
			} else {
				tx.cashAdjustmentAmt = l
				curState = txList
				stateChangeNum = i
				txs[len(txs)-1] = tx
				tx = transaction{}
			}
			continue
		}
		if curState == txList && i == stateChangeNum+1 {
			if l == "Total charges, credits and returns" {
				curState = end
				stateChangeNum = i
				continue
			}
			if pageBreakSentinel.MatchString(l) {
				curState = pageBreak
				stateChangeNum = i
				tx = transaction{}
				continue
			}
		}
		if curState == txList && i == stateChangeNum+1 {
			if l == "Promo Daily Cash" {
				curState = promo
				stateChangeNum = i
				tx = txs[len(txs)-1]
				continue
			}
			if l == "Daily Cash Adjustment" {
				curState = cashAdj
				stateChangeNum = i
				continue
			}
		}
		if curState == txList && i <= stateChangeNum+5 {
			if i == stateChangeNum+1 {
				tx.date = l
			} else if i == stateChangeNum+2 {
				tx.location = l
			} else if i == stateChangeNum+3 {
				if percentFormat.MatchString(l) {
					tx.cashBackPct = l
				} else {
					tx.transactionAmt = l
					stateChangeNum = i
					curState = txList
					continue
				}
			} else if i == stateChangeNum+4 {
				tx.cashBackAmt = l
			} else if i == stateChangeNum+5 {
				tx.transactionAmt = l
			}
		}
		if curState == txList && i == stateChangeNum+5 {
			txs = append(txs, tx)
			tx = transaction{}
			stateChangeNum = i
		}
	}
	return txs, nil
}

func writeCsv(txs []transaction) (string, error) {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	w.Write((transaction{}).header())
	for _, t := range txs {
		if err := w.Write(t.record()); err != nil {
			return "", err
		}
	}
	w.Flush()
	return b.String(), w.Error()
}

// ConvertPDFToCSV accepts a filename to an AppleCard transaction PDF and returns a
// CSV of transactions encoded as a single string
func ConvertPDFToCSV(filename string) (csvOut string, err error) {
	var lines []string
	var txList []transaction
	lines, err = readPDFLines(filename)
	if err != nil {
		return
	}
	txList, err = parseTransactions(lines)
	if err != nil {
		return
	}
	return writeCsv(txList)
}
