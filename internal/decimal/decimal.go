// Package decimal provides exact nonnegative decimal normalization and estimates.
package decimal

import (
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var syntax = regexp.MustCompile(`^\+?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)

// Parse accepts finite, nonnegative decimal source notation, including exponents.
func Parse(s string) (*big.Rat, error) {
	if len(s) > 4096 || !syntax.MatchString(s) {
		return nil, fmt.Errorf("invalid nonnegative decimal %q", s)
	}
	mantissa := s
	exponent := 0
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		mantissa = s[:i]
		n, err := strconv.Atoi(s[i+1:])
		if err != nil || n > 10000 || n < -10000 {
			return nil, fmt.Errorf("decimal exponent out of range")
		}
		exponent = n
	}

	// Calculate canonical size before creating a potentially huge rational.
	mantissa = strings.TrimPrefix(mantissa, "+")
	point := strings.IndexByte(mantissa, '.')
	if point < 0 {
		point = len(mantissa)
	}
	digits := strings.ReplaceAll(mantissa, ".", "")
	leading := len(digits) - len(strings.TrimLeft(digits, "0"))
	digits = strings.TrimLeft(digits, "0")
	digits = strings.TrimRight(digits, "0")
	if digits == "" {
		return new(big.Rat), nil
	}
	point += exponent - leading
	length := len(digits) + 1
	if point >= len(digits) {
		length = point
	} else if point <= 0 {
		length = 2 - point + len(digits)
	}
	if length > 4096 {
		return nil, fmt.Errorf("canonical decimal exceeds 4096 characters")
	}
	canonical := ""
	switch {
	case point >= len(digits):
		canonical = digits + strings.Repeat("0", point-len(digits))
	case point <= 0:
		canonical = "0." + strings.Repeat("0", -point) + digits
	default:
		canonical = digits[:point] + "." + digits[point:]
	}
	n, ok := new(big.Rat).SetString(canonical)
	if !ok {
		return nil, fmt.Errorf("invalid decimal %q", s)
	}
	return n, nil
}

// String serializes an exact terminating decimal without insignificant zeros.
func String(n *big.Rat) (string, error) {
	if n.Sign() < 0 {
		return "", fmt.Errorf("negative measurement")
	}
	den := new(big.Int).Set(n.Denom())
	twos, fives := 0, 0
	for _, factor := range []int64{2, 5} {
		f := big.NewInt(factor)
		for new(big.Int).Mod(den, f).Sign() == 0 {
			den.Quo(den, f)
			if factor == 2 {
				twos++
			} else {
				fives++
			}
		}
	}
	if den.Cmp(big.NewInt(1)) != 0 {
		return "", fmt.Errorf("nonterminating decimal")
	}
	places := twos
	if fives > places {
		places = fives
	}
	v := n.FloatString(places)
	if strings.Contains(v, ".") {
		v = strings.TrimRight(strings.TrimRight(v, "0"), ".")
	}
	if len(v) > 4096 {
		return "", fmt.Errorf("canonical decimal exceeds 4096 characters")
	}
	return v, nil
}
func Normalize(s string) (string, error) {
	n, err := Parse(s)
	if err != nil {
		return "", err
	}
	return String(n)
}
func Scale(s string, factor int64) (string, error) {
	n, err := Parse(s)
	if err != nil {
		return "", err
	}
	n.Mul(n, new(big.Rat).SetInt64(factor))
	return String(n)
}
func Median(samples []string) (string, error) {
	if len(samples) == 0 {
		return "", fmt.Errorf("empty samples")
	}
	values := make([]*big.Rat, len(samples))
	for i, s := range samples {
		n, err := Parse(s)
		if err != nil {
			return "", err
		}
		values[i] = n
	}
	sort.Slice(values, func(i, j int) bool { return values[i].Cmp(values[j]) < 0 })
	mid := len(values) / 2
	n := new(big.Rat).Set(values[mid])
	if len(values)%2 == 0 {
		n.Add(n, values[mid-1])
		n.Quo(n, big.NewRat(2, 1))
	}
	return String(n)
}
