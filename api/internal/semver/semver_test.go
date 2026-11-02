package semver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompareAndMax(t *testing.T) {
	assert.Equal(t, 1, Compare("1.10.0", "1.9.9"))
	assert.Equal(t, 0, Compare("2.0.0", "2.0.0"))
	assert.Equal(t, -1, Compare("bad", "0.0.1"))
	assert.Equal(t, "1.10.0", Max([]string{"1.2.0", "1.10.0", "1.9.0", "x"}))
	assert.Equal(t, "", Max(nil))
	assert.False(t, Valid("01.0.0"), "no leading zeros")
}

func TestSatisfies(t *testing.T) {
	cases := []struct {
		v, r string
		ok   bool
	}{
		{"1.4.2", "^1.0.0", true}, {"2.0.0", "^1.0.0", false}, {"0.9.3", "^0.9.0", true}, {"0.10.0", "^0.9.0", false},
		{"1.2.9", "~1.2.0", true}, {"1.3.0", "~1.2.0", false}, {"1.0.0", "1.0.0", true}, {"1.0.1", "1.0.0", false},
		{"3.0.0", ">=2.1.0", true}, {"2.0.9", ">=2.1.0", false}, {"9.9.9", "*", true}, {"1.0.0", "garbage", false},
	}
	for _, c := range cases {
		assert.Equal(t, c.ok, Satisfies(c.v, c.r), "%s in %s", c.v, c.r)
	}
	assert.True(t, ValidRange("^1.2.3"))
	assert.False(t, ValidRange("^1.2"))
}
