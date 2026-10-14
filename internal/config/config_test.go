package config

import "testing"

func TestRatingCalibration(t *testing.T) {
	t.Setenv("RATING_CALIBRATION_MATCHES", "")
	c, err := FromEnv()
	if err != nil || c.RatingCalibration != 10 {
		t.Fatalf("по умолчанию: %d, %v; ожидалось 10", c.RatingCalibration, err)
	}
	t.Setenv("RATING_CALIBRATION_MATCHES", "5")
	if c, err = FromEnv(); err != nil || c.RatingCalibration != 5 {
		t.Fatalf("RATING_CALIBRATION_MATCHES=5: %d, %v", c.RatingCalibration, err)
	}
	for _, v := range []string{"0", "-3", "abc", "2.5"} {
		t.Setenv("RATING_CALIBRATION_MATCHES", v)
		if _, err := FromEnv(); err == nil {
			t.Errorf("RATING_CALIBRATION_MATCHES=%q: ожидалась ошибка", v)
		}
	}
}
