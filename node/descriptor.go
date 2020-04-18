package infchat

import (
	"errors"
	"strings"
)

const (
	ChanPrefix = "/infinitychat/v0.1/channel/"
	DMPrefix   = "/infinity/v0.1/dm/"
)

// ExpandDescriptor converts infinitychat object descriptor from a short form
// that is used for display to the full-length form that is used in the
// protocol communication.
//
// Function is idempotent - if shortForm is already expanded, it does nothing.
//
// See DescriptorForDisplay for reverse conversion.
func ExpandDescriptor(shortForm string) (string, error) {
	switch {
	case strings.HasPrefix(shortForm, "/"):
		return shortForm, nil
	case strings.HasPrefix(shortForm, "#"):
		return ChanPrefix + shortForm[1:], nil
	case strings.HasPrefix(shortForm, "@"):
		return DMPrefix + shortForm[1:], nil
	default:
		return "", errors.New("unknown descriptor type")
	}
}

func DescriptorForDisplay(fullForm string) string {
	switch {
	case strings.HasPrefix(fullForm, "#"), strings.HasPrefix(fullForm, "@"):
		return fullForm
	case strings.HasPrefix(fullForm, ChanPrefix):
		return "#" + fullForm[len(ChanPrefix):]
	case strings.HasPrefix(fullForm, DMPrefix):
		return "@" + fullForm[len(DMPrefix):]
	default:
		return fullForm
	}
}
