package tenant

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FormatVersion 是本版本读取的快照格式；只有布局变化时才变，快照里多一张表不算新格式。
const FormatVersion = 1

// Manifest 是快照的 manifest.json，格式与 abby-pm 相同：
//
//	<dir>/manifest.json
//	<dir>/tables/<table>.jsonl   每行一个 JSON 对象，键为列名
//
// JSON Lines 便于在编辑器里阅读、在 PR 里比较差异。
type Manifest struct {
	Format int    `json:"format"`
	Name   string `json:"name"`
	// Description 说明数据从哪里来、是否虚构，给审核的人看。
	Description string `json:"description"`
	// TenantMigration 是切出这份快照的 schema 已应用的最新租户迁移版本；
	// 与代码里的版本不一致时拒绝导入（列对不上）。
	TenantMigration int64 `json:"tenant_migration"`
	// Tables 按导入顺序排列：被引用的表在引用它的表之前。
	Tables []Table `json:"tables"`
}

// Table 是一个表的数据文件。
type Table struct {
	Name string `json:"name"`
	File string `json:"file"`
	// Rows 和 SHA256 在写入前校验；Rows 在导入后再回读核对一次。
	Rows   int    `json:"rows"`
	SHA256 string `json:"sha256"`
}

// Snapshot 是已读取并校验过的快照。
type Snapshot struct {
	Dir      string
	Manifest Manifest
	// rows[i] 是 Tables[i] 的各行。
	rows [][]json.RawMessage
}

var reTableName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// tablesNeverLoaded 是快照不能携带的表：版本表属于被建的 schema；
// 用户与会话在 public schema，由测试账号单独写入，不随快照复制。
var tablesNeverLoaded = map[string]bool{
	"goose_db_version": true,
	"tenant":           true,
	"app_user":         true,
	"session":          true,
}

// LoadSnapshot 读取快照目录并按 manifest 校验；不连接数据库，有问题的快照在动任何
// schema 之前就被拒绝。
func LoadSnapshot(dir string) (*Snapshot, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("读取快照 manifest：%w", err)
	}
	var m Manifest
	dec := json.NewDecoder(bytes.NewReader(raw))
	// 手改 manifest 时拼错的键会被悄悄忽略，所以拒绝未知字段。
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("manifest.json：%w", err)
	}
	if m.Format != FormatVersion {
		return nil, fmt.Errorf("manifest.json 的格式是 %d，本版本读取格式 %d", m.Format, FormatVersion)
	}
	if m.TenantMigration <= 0 {
		return nil, fmt.Errorf("manifest.json 没有写明 tenant_migration")
	}
	if len(m.Tables) == 0 {
		return nil, fmt.Errorf("manifest.json 没有列出任何表")
	}
	s := &Snapshot{Dir: dir, Manifest: m, rows: make([][]json.RawMessage, len(m.Tables))}
	seen := map[string]bool{}
	for i, t := range m.Tables {
		if !reTableName.MatchString(t.Name) {
			return nil, fmt.Errorf("表名 %q 不是普通的表名", t.Name)
		}
		if tablesNeverLoaded[t.Name] {
			return nil, fmt.Errorf("表 %s 不能来自快照：它属于被建的租户或 public schema", t.Name)
		}
		if seen[t.Name] {
			return nil, fmt.Errorf("表 %s 列了两次", t.Name)
		}
		seen[t.Name] = true
		rows, err := readTable(dir, t)
		if err != nil {
			return nil, err
		}
		s.rows[i] = rows
	}
	return s, nil
}

// readTable 读取一个表的文件；拒绝跑出快照目录的路径、对不上的 SHA-256 或行数、
// 不是 JSON 对象的行。
func readTable(dir string, t Table) ([]json.RawMessage, error) {
	clean := filepath.Clean(t.File)
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("表 %s：文件 %q 在快照目录之外", t.Name, t.File)
	}
	raw, err := os.ReadFile(filepath.Join(dir, clean))
	if err != nil {
		return nil, fmt.Errorf("表 %s：%w", t.Name, err)
	}
	if got := FileSHA256(raw); !strings.EqualFold(got, t.SHA256) {
		return nil, fmt.Errorf("表 %s：%s 的 sha256 是 %s，manifest 写的是 %s", t.Name, t.File, got, t.SHA256)
	}
	rows, err := parseLines(raw)
	if err != nil {
		return nil, fmt.Errorf("表 %s：%s %w", t.Name, t.File, err)
	}
	if len(rows) != t.Rows {
		return nil, fmt.Errorf("表 %s：%s 有 %d 行，manifest 写的是 %d", t.Name, t.File, len(rows), t.Rows)
	}
	return rows, nil
}

// FileSHA256 返回内容的 SHA-256（十六进制小写）。
func FileSHA256(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// CountRows 返回 JSON Lines 内容的行数（忽略空行）；有不是 JSON 对象的行时返回错误。
func CountRows(raw []byte) (int, error) {
	rows, err := parseLines(raw)
	return len(rows), err
}

func parseLines(raw []byte) ([]json.RawMessage, error) {
	var rows []json.RawMessage
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for n := 1; sc.Scan(); n++ {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(line, &obj); err != nil {
			return nil, fmt.Errorf("第 %d 行不是 JSON 对象：%w", n, err)
		}
		rows = append(rows, json.RawMessage(append([]byte(nil), line...)))
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

// Rows 返回表 name 的行数；快照里没有这张表时返回 0。
func (s *Snapshot) Rows(name string) int {
	for i, t := range s.Manifest.Tables {
		if t.Name == name {
			return len(s.rows[i])
		}
	}
	return 0
}
