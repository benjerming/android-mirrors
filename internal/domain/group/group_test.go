package group

import (
	"strings"
	"testing"
)

// TestValidateName_Boundary 验证分组名长度边界与字符规则。
func TestValidateName_Boundary(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"", false},
		{"测试", true},
		{"a", true},
		{strings.Repeat("a", 32), true},
		{strings.Repeat("a", 33), false},
		{"含 空格", false},
		{"a-b_1", true},
		{"中文名_1", true},
		{"bad/slash", false},
	}
	for _, c := range cases {
		if got := ValidateName(c.in) == nil; got != c.ok {
			t.Errorf("ValidateName(%q) ok=%v want %v", c.in, got, c.ok)
		}
	}
}
