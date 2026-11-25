package utils

import (
	"errors"
	"time"
	_ "time/tzdata"
)

const timeZone = "Asia/Kolkata"

var (
	timeLocation, timeLocationErr = time.LoadLocation(timeZone)
)

func TimeNow() (time.Time, error) {
	if timeLocationErr != nil {
		return time.Time{}, errors.Join(ErrTimeLocationUnavailable, timeLocationErr)
	}
	if timeLocation == nil {
		return time.Time{}, ErrTimeLocationUnavailable
	}
	return time.Now().In(timeLocation), nil
}
