package tenant

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"example.com/dailylog/daily"
)

var update = flag.Bool("update", false, "按 tables/ 重写 Golden 快照 manifest 的 rows 和 sha256")

const goldenDir = "../../testdata/golden"

// TestGoldenManifest 校验 Golden 快照能通过 LoadSnapshot；带 -update 时先重算 manifest。
func TestGoldenManifest(t *testing.T) {
	if *update {
		path := filepath.Join(goldenDir, "manifest.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var m Manifest
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		for i, tb := range m.Tables {
			data, err := os.ReadFile(filepath.Join(goldenDir, tb.File))
			if err != nil {
				t.Fatal(err)
			}
			n, err := CountRows(data)
			if err != nil {
				t.Fatalf("%s: %v", tb.File, err)
			}
			m.Tables[i].Rows = n
			m.Tables[i].SHA256 = FileSHA256(data)
		}
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(m); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	snap, err := LoadSnapshot(goldenDir)
	if err != nil {
		t.Fatal(err)
	}
	if got := snap.Rows("job"); got != 3 {
		t.Errorf("job 行数 = %d, want 3", got)
	}
	if got := snap.Rows("report"); got != 30 {
		t.Errorf("report 行数 = %d, want 30", got)
	}
}

// TestGoldenCoverage 用 daily 包检查 Golden 快照覆盖了设计要求的各种情况。
func TestGoldenCoverage(t *testing.T) {
	snap, err := LoadSnapshot(goldenDir)
	if err != nil {
		t.Fatal(err)
	}
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	reports := map[int64]*daily.Report{}
	var order []int64
	for _, raw := range snap.rows[1] {
		var r struct {
			ID           int64      `json:"id"`
			JobID        string     `json:"job_id"`
			BusinessDate string     `json:"business_date"`
			SubmittedAt  *time.Time `json:"submitted_at"`
			Locked       bool       `json:"locked"`
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			t.Fatal(err)
		}
		d, err := time.ParseInLocation("2006-01-02", r.BusinessDate, loc)
		if err != nil {
			t.Fatal(err)
		}
		if d.Year() != 2026 || d.Month() != time.September {
			t.Errorf("日报 %d 的业务日期 %s 不在 2026-09", r.ID, r.BusinessDate)
		}
		rep := daily.NewReport(r.JobID, d)
		if r.SubmittedAt != nil {
			rep.SubmittedAt = *r.SubmittedAt
		}
		rep.Locked = r.Locked
		reports[r.ID] = rep
		order = append(order, r.ID)
	}
	for _, raw := range snap.rows[2] {
		var c struct {
			ReportID int64   `json:"report_id"`
			Worker   string  `json:"worker"`
			Hours    float64 `json:"hours"`
		}
		if err := json.Unmarshal(raw, &c); err != nil {
			t.Fatal(err)
		}
		if err := reports[c.ReportID].Update(map[string]float64{c.Worker: c.Hours}); err != nil {
			t.Fatal(err)
		}
	}
	withNotes := map[int64]bool{}
	for _, raw := range snap.rows[3] {
		var n struct {
			ReportID int64 `json:"report_id"`
		}
		if err := json.Unmarshal(raw, &n); err != nil {
			t.Fatal(err)
		}
		withNotes[n.ReportID] = true
	}

	count := map[string]int{}
	for _, id := range order {
		r := reports[id]
		switch {
		case !r.Submitted():
			count["未提交"]++
		case r.DaysLate() == 0:
			count["按时提交"]++
		}
		if d := r.DaysLate(); d >= 1 && d <= 3 {
			count["迟交"+string(rune('0'+d))+"天"]++
		}
		if r.Locked {
			count["已锁定"]++
		}
		if withNotes[id] {
			count["有备注"]++
		}
		if len(r.Crew) == 0 {
			count["无工时"]++
		}
	}
	for _, c := range []string{"未提交", "按时提交", "迟交1天", "迟交2天", "迟交3天", "已锁定", "有备注", "无工时"} {
		t.Run(c, func(t *testing.T) {
			if count[c] == 0 {
				t.Errorf("Golden 快照里没有%s的日报", c)
			}
		})
	}
}
