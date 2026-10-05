package auth

// Role 是用户角色。
type Role string

// 四种角色，见 docs/design.md 5.1 节。
const (
	Admin   Role = "admin"
	Manager Role = "manager"
	Field   Role = "field"
	Finance Role = "finance"
)

// Roles 按固定顺序列出全部角色。
var Roles = []Role{Admin, Manager, Field, Finance}

// Valid 报告 r 是否是已知角色。
func (r Role) Valid() bool {
	_, ok := permissions[r]
	return ok
}

// Action 是需要授权的操作。
type Action string

// 操作，见 docs/api.md 的权限表。
const (
	// View 查看台账、详情、工地。
	View Action = "view"
	// EditReport 新建日报、改工时、提交。
	EditReport Action = "edit_report"
	// Lock 锁定日报。
	Lock Action = "lock"
	// AddNote 追加备注。
	AddNote Action = "add_note"
	// ViewSummary 查看汇总。
	ViewSummary Action = "view_summary"
)

var permissions = map[Role]map[Action]bool{
	Admin:   {View: true, EditReport: true, Lock: true, AddNote: true, ViewSummary: true},
	Manager: {View: true, Lock: true, AddNote: true, ViewSummary: true},
	Field:   {View: true, EditReport: true, AddNote: true},
	Finance: {View: true, ViewSummary: true},
}

// Can 报告角色 r 是否允许执行操作 a；未知角色什么都不允许。
func (r Role) Can(a Action) bool {
	return permissions[r][a]
}
