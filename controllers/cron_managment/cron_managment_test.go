package cronmanagment

import "testing"

func TestTrue(t *testing.T) {
	got, _ := IsInTimezone("0 13 * * 1-5", "7h21m33s")
	if got != true {
		t.Errorf("inTimezoneOrNot() = %t; want true", got)
	}
}

func TestFalse(t *testing.T) {
	got, _ := IsInTimezone("0 10 * * 1-5", "2h21m33s")
	if got != false {
		t.Errorf("inTimezoneOrNot() = %t; want false", got)
	}
}

func TestBasicSuperiorToOneDay(t *testing.T) {
	got, _ := IsInTimezone("0 13 * * 1-5", "30h21m33s")
	if got != true {
		t.Errorf("inTimezoneOrNot() = %t; want true", got)
	}
}

func TestNotMonday(t *testing.T) {
	got, _ := IsInTimezone("0 13 * * 2-5", "30h21m33s")
	if got != false {
		t.Errorf("inTimezoneOrNot() = %t; want false", got)
	}
}

func TestInvalidDuration(t *testing.T) {
	got, err := IsInTimezone("0 13 * * 2-5", "30h21m33ss")
	if err == nil {
		t.Errorf("inTimezoneOrNot() = %t; want invalid duration", got)
	}
}
func TestInvalidTime(t *testing.T) {
	got, err := IsInTimezone("0 13 * * 2-5 *", "30h21m33s")
	if err == nil {
		t.Errorf("inTimezoneOrNot() = %t; want invalid time argument", got)
	}
}
