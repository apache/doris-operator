package utils

import "errors"

func MergeError(err1 error, err2 error) error {
	if err1 == nil && err2 == nil {
		return nil
	}
	if err1 == nil {
		return err2
	}
	if err2 == nil {
		return err1
	}
	return errors.New(err1.Error() + "; " + err2.Error())
}
