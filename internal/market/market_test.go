package market

import "testing"

func TestAbbr(t *testing.T) {
	cases := []struct {
		code string
		want string
	}{
		{"17", "SH"},
		{"33", "SZ"},
		{"151", "BJ"},
		{"71", "BJ"}, // 旧码兼容
		{"18", "KC"},
		{"9999", "9999"}, // 未知原样返回
		{"", ""},
	}
	for _, c := range cases {
		if got := Abbr(c.code); got != c.want {
			t.Errorf("Abbr(%q) = %q, want %q", c.code, got, c.want)
		}
	}
}

func TestCode(t *testing.T) {
	cases := []struct {
		abbr    string
		want    string
		wantErr bool
	}{
		{"SH", "17", false},
		{"sh", "17", false}, // 大小写不敏感
		{"SZ", "33", false},
		{"BJ", "151", false},
		{"KC", "18", false},
		{"HK", "55", false},
		{"US", "61", false},
		{"XX", "", true},
		{"", "", true},
	}
	for _, c := range cases {
		got, err := Code(c.abbr)
		if c.wantErr {
			if err == nil {
				t.Errorf("Code(%q) 期望报错，实际返回 %q", c.abbr, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("Code(%q) 意外报错: %v", c.abbr, err)
		}
		if got != c.want {
			t.Errorf("Code(%q) = %q, want %q", c.abbr, got, c.want)
		}
	}
}
