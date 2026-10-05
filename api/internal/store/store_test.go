package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/dailylog/api/internal/db"
	"example.com/dailylog/api/internal/store"
	"example.com/dailylog/api/internal/testdb"
	"example.com/dailylog/daily"
)

const slot = "dlpv991"

var shanghai = mustLoc("Asia/Shanghai")

func mustLoc(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

func date(s string) time.Time {
	t, err := store.Date(s, shanghai)
	if err != nil {
		panic(err)
	}
	return t
}

// inTx 在租户事务里执行 fn；fn 返回错误时测试失败。
func inTx(t *testing.T, pool *pgxpool.Pool, fn func(ctx context.Context, st *store.Store) error) {
	t.Helper()
	ctx := context.Background()
	err := db.InTenant(ctx, pool, slot, func(tx pgx.Tx) error {
		return fn(ctx, store.New(tx, shanghai))
	})
	if err != nil {
		t.Fatal(err)
	}
}

func setup(t *testing.T) *pgxpool.Pool {
	pool := testdb.Pool(t)
	testdb.Seed(t, slot)
	return pool
}

func TestJobs(t *testing.T) {
	pool := setup(t)
	inTx(t, pool, func(ctx context.Context, st *store.Store) error {
		jobs, err := st.Jobs(ctx)
		if err != nil {
			return err
		}
		want := []string{"J-24001", "J-24002", "J-24003"}
		if len(jobs) != len(want) {
			t.Fatalf("jobs = %v", jobs)
		}
		for i, j := range jobs {
			if j.ID != want[i] || j.Name == "" || j.Address == "" {
				t.Errorf("jobs[%d] = %+v", i, j)
			}
		}
		return nil
	})
}

func TestReportsFilter(t *testing.T) {
	pool := setup(t)
	tests := []struct {
		name string
		f    store.Filter
		want int
	}{
		{"不限", store.Filter{}, 30},
		{"按工地", store.Filter{JobID: "J-24002"}, 10},
		{"不存在的工地", store.Filter{JobID: "J-99999"}, 0},
		{"起始日期（闭区间）", store.Filter{From: date("2026-09-09")}, 6},
		{"截止日期（闭区间）", store.Filter{To: date("2026-09-01")}, 3},
		{"工地加日期范围", store.Filter{JobID: "J-24001", From: date("2026-09-03"), To: date("2026-09-05")}, 3},
		{"起始晚于截止", store.Filter{From: date("2026-09-05"), To: date("2026-09-04")}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inTx(t, pool, func(ctx context.Context, st *store.Store) error {
				recs, err := st.Reports(ctx, tt.f)
				if err != nil {
					return err
				}
				if len(recs) != tt.want {
					t.Errorf("len = %d, want %d", len(recs), tt.want)
				}
				return nil
			})
		})
	}
}

func TestReportRoundTrip(t *testing.T) {
	pool := setup(t)
	// id 与 Golden 快照对应：1 = J-24001 09-01（已锁定、2 条备注），15 = J-24002 09-05
	// （次日 07:30 提交：按 UTC 是当天，按业务时区迟交 1 天），17 = J-24002 09-07（已提交、无工时）。
	tests := []struct {
		id        int64
		wantJob   string
		wantDate  string
		wantLate  int
		wantHours float64
		wantLock  bool
		wantNotes []string
	}{
		{1, "J-24001", "2026-09-01", 0, 33.5, true, []string{"基础垫层浇筑完成。", "已结算，锁定。"}},
		{15, "J-24002", "2026-09-05", 1, 25.5, false, nil},
		{17, "J-24002", "2026-09-07", 0, 0, false, []string{"雨天停工，无人出勤。"}},
	}
	for _, tt := range tests {
		t.Run(tt.wantJob+" "+tt.wantDate, func(t *testing.T) {
			inTx(t, pool, func(ctx context.Context, st *store.Store) error {
				rec, err := st.Report(ctx, tt.id, false)
				if err != nil {
					return err
				}
				r := rec.Report
				if r.JobID != tt.wantJob || r.BusinessDate.Format(store.DateLayout) != tt.wantDate {
					t.Errorf("report = %s %s", r.JobID, r.BusinessDate)
				}
				if r.BusinessDate.Location() != shanghai {
					t.Errorf("业务日期的时区 = %v, want Asia/Shanghai", r.BusinessDate.Location())
				}
				if got := r.DaysLate(); got != tt.wantLate {
					t.Errorf("DaysLate = %d, want %d", got, tt.wantLate)
				}
				if got := r.TotalHours(); got != tt.wantHours {
					t.Errorf("TotalHours = %v, want %v", got, tt.wantHours)
				}
				if r.Locked != tt.wantLock {
					t.Errorf("Locked = %v, want %v", r.Locked, tt.wantLock)
				}
				if len(r.Notes) != len(tt.wantNotes) || len(rec.Notes) != len(tt.wantNotes) {
					t.Fatalf("notes = %v", r.Notes)
				}
				for i := range tt.wantNotes {
					if r.Notes[i] != tt.wantNotes[i] || rec.Notes[i].Body != tt.wantNotes[i] {
						t.Errorf("notes[%d] = %q", i, r.Notes[i])
					}
				}
				return nil
			})
		})
	}

	t.Run("不存在", func(t *testing.T) {
		inTx(t, pool, func(ctx context.Context, st *store.Store) error {
			if _, err := st.Report(ctx, 9999, false); !errors.Is(err, store.ErrNotFound) {
				t.Errorf("err = %v, want ErrNotFound", err)
			}
			return nil
		})
	})
}

func TestCreateAndSave(t *testing.T) {
	pool := setup(t)
	now := time.Date(2026, 9, 11, 1, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		job     string
		date    string
		wantErr error
	}{
		{"新建", "J-24001", "2026-09-11", nil},
		{"同一工地同一天重复", "J-24001", "2026-09-01", store.ErrExists},
		{"工地不存在", "J-99999", "2026-09-11", store.ErrNoJob},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inTx(t, pool, func(ctx context.Context, st *store.Store) error {
				id, err := st.Create(ctx, daily.NewReport(tt.job, date(tt.date)), now)
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				if err == nil && id <= 30 {
					t.Errorf("id = %d：序列没有越过快照里的 id", id)
				}
				return nil
			})
		})
	}

	var id int64
	inTx(t, pool, func(ctx context.Context, st *store.Store) error {
		recs, err := st.Reports(ctx, store.Filter{From: date("2026-09-11")})
		if err != nil || len(recs) != 1 {
			t.Fatalf("recs = %v, err = %v", recs, err)
		}
		id = recs[0].ID
		if err := st.SaveCrew(ctx, id, map[string]float64{"甲": 8, "乙": 4}); err != nil {
			return err
		}
		// 第二次整体替换：乙被去掉。
		if err := st.SaveCrew(ctx, id, map[string]float64{"甲": 6, "丙": 2.5}); err != nil {
			return err
		}
		rec, err := st.Report(ctx, id, true)
		if err != nil {
			return err
		}
		if err := rec.Report.Submit(now); err != nil {
			return err
		}
		rec.Report.Lock()
		if err := st.SaveState(ctx, id, &rec.Report); err != nil {
			return err
		}
		if err := st.AddNote(ctx, id, "第一条", 0, "某人", now); err != nil {
			return err
		}
		return st.AddNote(ctx, id, "第二条", 0, "某人", now.Add(time.Minute))
	})
	inTx(t, pool, func(ctx context.Context, st *store.Store) error {
		rec, err := st.Report(ctx, id, false)
		if err != nil {
			return err
		}
		r := rec.Report
		if len(r.Crew) != 2 || r.Crew["甲"] != 6 || r.Crew["丙"] != 2.5 {
			t.Errorf("crew = %v", r.Crew)
		}
		if !r.SubmittedAt.Equal(now) || !r.Locked {
			t.Errorf("submitted = %v, locked = %v", r.SubmittedAt, r.Locked)
		}
		if got := r.LatestNotes(2); len(got) != 2 || got[0] != "第二条" {
			t.Errorf("LatestNotes = %v", got)
		}
		return nil
	})
}
