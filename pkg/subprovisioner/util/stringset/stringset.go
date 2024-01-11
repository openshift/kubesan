// SPDX-License-Identifier: Apache-2.0

package stringset

import "strings"

type StringSet map[string]struct{}

func FromString(s string) StringSet {
	set := map[string]struct{}{}

	if s != "" {
		for _, item := range strings.Split(s, ",") {
			set[item] = struct{}{}
		}
	}

	return set
}

func (ss StringSet) ToString() string {
	var builder strings.Builder
	empty := true

	for item := range ss {
		if !empty {
			builder.WriteRune(',')
		}
		builder.WriteString(item)
		empty = false
	}

	return builder.String()
}

func (ss StringSet) Insert(item string) {
	ss[item] = struct{}{}
}

func (ss StringSet) Remove(item string) {
	delete(ss, item)
}

func Len(s string) int {
	return len(FromString(s))
}

func Insert(s string, item string) string {
	set := FromString(s)
	set.Insert(item)
	return set.ToString()
}

func Remove(s string, item string) string {
	set := FromString(s)
	set.Remove(item)
	return set.ToString()
}
