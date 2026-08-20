package credentials

import "testing"

func TestSentinel(t *testing.T) {
	tests := []struct {
		field string
		want  string
	}{
		{field: "email", want: "user@example.org"},
		{field: "username", want: "cooljohnny1567"},
		{field: "password", want: "mycoolpassword"},
		{field: "mfa", want: "999779"},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			got, err := Sentinel(tt.field)
			if err != nil {
				t.Fatalf("Sentinel(%q) returned an error: %v", tt.field, err)
			}
			if got != tt.want {
				t.Errorf("Sentinel(%q) = %q, want %q", tt.field, got, tt.want)
			}
		})
	}
}

func TestSentinelRejectsUnknownField(t *testing.T) {
	if _, err := Sentinel("token"); err == nil {
		t.Fatal("Sentinel(\"token\") did not return an error")
	}
}
