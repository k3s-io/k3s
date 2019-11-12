package units

import (
	"fmt"
	"strconv"
)

// ParseByteSizeString parses a human representation of an amount of
// data into a number of bytes
func ParseByteSizeString(input string) (int64, error) {
	// Empty input
	if input == "" {
		return 0, nil
	}

	// Find where the suffix begins
	suffixLen := 0
	for i, chr := range []byte(input) {
		_, err := strconv.Atoi(string([]byte{chr}))
		if err != nil {
			suffixLen = len(input) - i
			break
		}
	}

	if suffixLen == len(input) {
		return -1, fmt.Errorf("Invalid value: %s", input)
	}

	// Extract the suffix
	suffix := input[len(input)-suffixLen:]

	// Extract the value
	value := input[0 : len(input)-suffixLen]
	valueInt, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return -1, fmt.Errorf("Invalid integer: %s", input)
	}

	// Figure out the multiplicator
	multiplicator := int64(0)
	switch suffix {
	case "", "B", " bytes":
		multiplicator = 1
	case "kB":
		multiplicator = 1000
	case "MB":
		multiplicator = 1000 * 1000
	case "GB":
		multiplicator = 1000 * 1000 * 1000
	case "TB":
		multiplicator = 1000 * 1000 * 1000 * 1000
	case "PB":
		multiplicator = 1000 * 1000 * 1000 * 1000 * 1000
	case "EB":
		multiplicator = 1000 * 1000 * 1000 * 1000 * 1000 * 1000
	case "KiB":
		multiplicator = 1024
	case "MiB":
		multiplicator = 1024 * 1024
	case "GiB":
		multiplicator = 1024 * 1024 * 1024
	case "TiB":
		multiplicator = 1024 * 1024 * 1024 * 1024
	case "PiB":
		multiplicator = 1024 * 1024 * 1024 * 1024 * 1024
	case "EiB":
		multiplicator = 1024 * 1024 * 1024 * 1024 * 1024 * 1024
	default:
		return -1, fmt.Errorf("Invalid value: %s", input)
	}

	return valueInt * multiplicator, nil
}

// ParseBitSizeString parses a human representation of an amount of
// data into a number of bits
func ParseBitSizeString(input string) (int64, error) {
	// Empty input
	if input == "" {
		return 0, nil
	}

	// Find where the suffix begins
	suffixLen := 0
	for i, chr := range []byte(input) {
		_, err := strconv.Atoi(string([]byte{chr}))
		if err != nil {
			suffixLen = len(input) - i
			break
		}
	}

	if suffixLen == len(input) {
		return -1, fmt.Errorf("Invalid value: %s", input)
	}

	// Extract the suffix
	suffix := input[len(input)-suffixLen:]

	// Extract the value
	value := input[0 : len(input)-suffixLen]
	valueInt, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return -1, fmt.Errorf("Invalid integer: %s", input)
	}

	// Figure out the multiplicator
	multiplicator := int64(0)
	switch suffix {
	case "", "bit":
		multiplicator = 1
	case "kbit":
		multiplicator = 1000
	case "Mbit":
		multiplicator = 1000 * 1000
	case "Gbit":
		multiplicator = 1000 * 1000 * 1000
	case "Tbit":
		multiplicator = 1000 * 1000 * 1000 * 1000
	case "Pbit":
		multiplicator = 1000 * 1000 * 1000 * 1000 * 1000
	case "Ebit":
		multiplicator = 1000 * 1000 * 1000 * 1000 * 1000 * 1000
	case "Kibit":
		multiplicator = 1024
	case "Mibit":
		multiplicator = 1024 * 1024
	case "Gibit":
		multiplicator = 1024 * 1024 * 1024
	case "Tibit":
		multiplicator = 1024 * 1024 * 1024 * 1024
	case "Pibit":
		multiplicator = 1024 * 1024 * 1024 * 1024 * 1024
	case "Eibit":
		multiplicator = 1024 * 1024 * 1024 * 1024 * 1024 * 1024

	default:
		return -1, fmt.Errorf("Unsupported suffix: %s", suffix)
	}

	return valueInt * multiplicator, nil
}

// GetByteSizeString takes a number of bytes and precision and returns a
// human representation of the amount of data
func GetByteSizeString(input int64, precision uint) string {
	if input < 1000 {
		return fmt.Sprintf("%dB", input)
	}

	value := float64(input)

	for _, unit := range []string{"kB", "MB", "GB", "TB", "PB", "EB"} {
		value = value / 1000
		if value < 1000 {
			return fmt.Sprintf("%.*f%s", precision, value, unit)
		}
	}

	return fmt.Sprintf("%.*fEB", precision, value)
}
