package version

import "testing"

func TestParseAndCompare(t *testing.T) {
	t.Parallel()
	left, err := Parse("v1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	right, err := Parse("1.3.0")
	if err != nil {
		t.Fatal(err)
	}
	if left.String() != "1.2.3" {
		t.Fatalf("String() = %q", left.String())
	}
	if Compare(left, right) >= 0 {
		t.Fatalf("Compare(%v,%v) should be negative", left, right)
	}
}

func TestAtLeast(t *testing.T) {
	t.Parallel()
	if err := AtLeast("1.2.0", "1.1.9"); err != nil {
		t.Fatal(err)
	}
	if err := AtLeast("1.0.0", "1.1.0"); err == nil {
		t.Fatal("expected old version error")
	}
}

func TestParseRejectsInvalidVersion(t *testing.T) {
	t.Parallel()
	if _, err := Parse("1.2"); err == nil {
		t.Fatal("expected parse error")
	}
}
