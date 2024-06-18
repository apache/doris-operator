package fdb

import (
	"errors"
	"fmt"
	"strings"
)

// use ":" as IPS to split image as baseimage and version.
func imageSplit(image string) (baseImage, version string, err error) {
	isa := strings.Split(image, ":")
	if len(isa) == 0 {
		err = errors.New(fmt.Sprintf("the image = %s format is not provided. please reference docker format.", image))
		return
	}

	baseImage = isa[0]
	version = isa[1]
	return
}
