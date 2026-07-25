package solution

import "testing"

func TestVValid(t *testing.T) {
	valid := []string{
		"0.0.0.0",
		"255.255.255.255",
		"192.168.1.1",
		"1.2.3.4",
		"127.0.0.1",
		"8.8.8.8",
		"10.0.0.255",
		"100.64.0.0",
	}
	for _, s := range valid {
		if !IsValidIPv4(s) {
			t.Errorf("IsValidIPv4(%q) = false, want true", s)
		}
	}
}

func TestVInvalid(t *testing.T) {
	invalid := []string{
		"256.1.1.1",  // octet out of range
		"1.1.1",      // too few
		"1.1.1.1.1",  // too many
		"01.1.1.1",   // leading zero
		"1.1.1.256",  // out of range
		"1.1.1.-1",   // negative sign not a digit
		"a.b.c.d",    // non-numeric
		"192.168.1.", // empty last octet
		"",           // empty string
	}
	for _, s := range invalid {
		if IsValidIPv4(s) {
			t.Errorf("IsValidIPv4(%q) = true, want false", s)
		}
	}
}

func TestHLeadingZeros(t *testing.T) {
	if IsValidIPv4("00.0.0.0") {
		t.Error(`"00.0.0.0" should be invalid`)
	}
	if IsValidIPv4("0.0.0.00") {
		t.Error(`"0.0.0.00" should be invalid`)
	}
	if !IsValidIPv4("0.0.0.0") {
		t.Error(`"0.0.0.0" should be valid`)
	}
	if IsValidIPv4("192.168.001.1") {
		t.Error(`"192.168.001.1" should be invalid`)
	}
}

func TestHBoundaryOctets(t *testing.T) {
	if !IsValidIPv4("255.255.255.255") {
		t.Error("255 boundary should be valid")
	}
	if IsValidIPv4("255.255.255.256") {
		t.Error("256 should be invalid")
	}
	// Three-digit under 255.
	if !IsValidIPv4("199.199.199.199") {
		t.Error("199.x should be valid")
	}
}

func TestHWhitespaceAndSigns(t *testing.T) {
	bad := []string{
		" 1.1.1.1",
		"1.1.1.1 ",
		"1. 1.1.1",
		"+1.1.1.1",
		"1.1.1.1\n",
		"1..1.1",
		".1.1.1",
		"1.1.1.1.",
		"...",
	}
	for _, s := range bad {
		if IsValidIPv4(s) {
			t.Errorf("IsValidIPv4(%q) = true, want false", s)
		}
	}
}

func TestHNonASCIIDigits(t *testing.T) {
	// Full-width digits and other unicode must not be accepted.
	if IsValidIPv4("１.1.1.1") {
		t.Error("full-width digit should be invalid")
	}
	if IsValidIPv4("1.1.1.१") {
		t.Error("devanagari digit should be invalid")
	}
}

func TestHFourDigitOctet(t *testing.T) {
	if IsValidIPv4("1000.1.1.1") {
		t.Error("four-digit octet should be invalid")
	}
}
