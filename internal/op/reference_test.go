package op

import "testing"

func TestParseReference(t *testing.T) {
	cases := []struct {
		in    string
		ok    bool
		vault string
		item  string
		field string
	}{
		{"op://Private/server-prod", true, "Private", "server-prod", ""},
		{"op://DevOps/my-server/password", true, "DevOps", "my-server", "password"},
		{"op://Private/server-prod/section/field", true, "Private", "server-prod", "section/field"},
		{"not-a-reference", false, "", "", ""},
		{"op://Private", false, "", "", ""},
		{"op:///item", false, "", "", ""},
		{"", false, "", "", ""},
	}
	for _, tc := range cases {
		r, err := ParseReference(tc.in)
		if (err == nil) != tc.ok {
			t.Errorf("ParseReference(%q) ok=%v want=%v err=%v", tc.in, err == nil, tc.ok, err)
			continue
		}
		if !tc.ok {
			continue
		}
		if r.Vault != tc.vault || r.Item != tc.item || r.Field != tc.field {
			t.Errorf("ParseReference(%q) = %+v", tc.in, r)
		}
	}
}

func TestReference_String(t *testing.T) {
	r := Reference{Vault: "Private", Item: "x"}
	if r.String() != "op://Private/x" {
		t.Fatalf("got %q", r.String())
	}
	r.Field = "password"
	if r.String() != "op://Private/x/password" {
		t.Fatalf("got %q", r.String())
	}
}
