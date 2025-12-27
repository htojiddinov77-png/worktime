package api

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)



func readString(qs url.Values, key string, defaultValue string) string {
	s := qs.Get(key)

	if s == "" {
		return defaultValue
	}

	return s
}


func readCSV(qs url.Values, key string, defaultValue []string) []string {
	csv := qs.Get(key)

	if csv == "" {
		return defaultValue
	}
	// parse the value into a []string slice and return
	return strings.Split(csv, ",")
}


func readInt(qs url.Values, key string, defaultValue int) int {
	s := qs.Get(key)

	if s == "" {
		return defaultValue
	}

	i, err := strconv.Atoi(s)
	if err != nil {
		fmt.Errorf("Must be integer value")
		return defaultValue
	}

	return i
}