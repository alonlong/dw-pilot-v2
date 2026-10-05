package tenant

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"example.com/dailylog/api/internal/auth"
)

// Account 是一个测试账号。密码不在文件里，统一取 TENANT_TEST_PASSWORD。
type Account struct {
	Email string    `json:"email"`
	Name  string    `json:"name"`
	Role  auth.Role `json:"role"`
}

// Accounts 是一个租户的测试账号文件：
//
//	{"tenant": "dlpv1", "accounts": [{"email": "…", "name": "…", "role": "admin"}]}
//
// 与快照分开：一份快照给所有槽位用，账号按槽位各不相同。
type Accounts struct {
	// Tenant 是这些账号所属的租户，必须与要建的租户一致。
	Tenant   string    `json:"tenant"`
	Accounts []Account `json:"accounts"`
}

// defaultNames 是默认账号的显示名。
var defaultNames = map[auth.Role]string{
	auth.Admin:   "测试管理员",
	auth.Manager: "测试项目经理",
	auth.Field:   "测试现场",
	auth.Finance: "测试财务",
}

// DefaultAccounts 按四种角色生成默认账号：admin@<tenant>.test、manager@…、field@…、finance@…。
func DefaultAccounts(tenant string) *Accounts {
	a := &Accounts{Tenant: tenant}
	for _, r := range auth.Roles {
		a.Accounts = append(a.Accounts, Account{
			Email: string(r) + "@" + tenant + ".test",
			Name:  defaultNames[r],
			Role:  r,
		})
	}
	return a
}

// LoadAccounts 读取账号文件并逐个校验；文件里的租户必须是 tenant。
func LoadAccounts(path, tenant string) (*Accounts, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取账号文件：%w", err)
	}
	var a Accounts
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return nil, fmt.Errorf("%s：%w", path, err)
	}
	if a.Tenant != tenant {
		return nil, fmt.Errorf("%s 是租户 %q 的账号，不是 %q", path, a.Tenant, tenant)
	}
	if err := a.validate(); err != nil {
		return nil, fmt.Errorf("%s：%w", path, err)
	}
	return &a, nil
}

func (a *Accounts) validate() error {
	if len(a.Accounts) == 0 {
		return fmt.Errorf("没有账号")
	}
	seen := map[string]bool{}
	for i := range a.Accounts {
		acct := &a.Accounts[i]
		acct.Email = auth.NormalizeEmail(acct.Email)
		acct.Name = strings.TrimSpace(acct.Name)
		at := strings.IndexByte(acct.Email, '@')
		if at <= 0 || at == len(acct.Email)-1 {
			return fmt.Errorf("第 %d 个账号的邮箱 %q 不对", i+1, acct.Email)
		}
		if acct.Name == "" {
			return fmt.Errorf("账号 %s 没有名字", acct.Email)
		}
		if !acct.Role.Valid() {
			return fmt.Errorf("账号 %s 的角色 %q 不对", acct.Email, acct.Role)
		}
		if seen[acct.Email] {
			return fmt.Errorf("邮箱 %s 出现了两次", acct.Email)
		}
		seen[acct.Email] = true
	}
	return nil
}
