package evaluation

import (
	"bytes"
	"testing"
)

var testKey = bytes.Repeat([]byte("k"), MinKeyLength)

func TestMatchesIgnoresCaseAndSurroundingSpace(t *testing.T) {
	expected := Digest(testKey, "D935e3c2d8c7955277d7f1c453a62c26")

	for _, answer := range []string{
		"D935e3c2d8c7955277d7f1c453a62c26",
		"d935e3c2d8c7955277d7f1c453a62c26",
		"  D935E3C2D8C7955277D7F1C453A62C26\n",
	} {
		if !Matches(testKey, answer, expected[:]) {
			t.Errorf("Matches(%q) = false; want true", answer)
		}
	}
}

func TestMatchesRejectsOtherAnswers(t *testing.T) {
	expected := Digest(testKey, "41.133")

	for _, answer := range []string{"41.13", "41. 133", "", "41.1330"} {
		if Matches(testKey, answer, expected[:]) {
			t.Errorf("Matches(%q) = true; want false", answer)
		}
	}
}

func TestDigestDependsOnKey(t *testing.T) {
	other := bytes.Repeat([]byte("x"), MinKeyLength)
	if Digest(testKey, "TCP") == Digest(other, "TCP") {
		t.Fatal("digests under different keys are equal")
	}
}
