package render

import (
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

var integerNotation = regexp.MustCompile(`^(0|[1-9][0-9]*)(?:\.([0-9]+))?(?:[eE]([+-]?[0-9]+))?$`)

// rowLimit caps before expanding exponent notation, keeping huge valid JSON
// integers meaningful without allocating their zero-filled decimal expansion.
func rowLimit(text string, available int) (int, error) {
	invalid := func() (int, error) { return 0, fmt.Errorf("report.max_rows must be a nonnegative integer") }
	parts := integerNotation.FindStringSubmatch(text)
	if parts == nil {
		return invalid()
	}
	digits := strings.TrimLeft(parts[1]+parts[2], "0")
	if digits == "" {
		return available, nil
	}
	exponent := new(big.Int)
	if parts[3] != "" {
		if _, ok := exponent.SetString(parts[3], 10); !ok {
			return invalid()
		}
	}
	scale := new(big.Int).Sub(exponent, big.NewInt(int64(len(parts[2]))))
	if scale.Sign() < 0 {
		trailing := len(digits) - len(strings.TrimRight(digits, "0"))
		remove := new(big.Int).Neg(scale)
		if remove.Cmp(big.NewInt(int64(trailing))) > 0 {
			return invalid()
		}
		digits = digits[:len(digits)-int(remove.Int64())]
		scale.SetInt64(0)
	}
	size := new(big.Int).Add(scale, big.NewInt(int64(len(digits))))
	if size.Cmp(big.NewInt(int64(len(strconv.Itoa(available))))) > 0 {
		return available, nil
	}
	digits += strings.Repeat("0", int(scale.Int64()))
	number, err := strconv.Atoi(digits)
	if err != nil {
		return invalid()
	}
	return min(number, available), nil
}
