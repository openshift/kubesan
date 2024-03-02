// SPDX-License-Identifier: Apache-2.0

package slices

func AppendUnique[T comparable](list []T, elem T) []T {
	for _, e := range list {
		if e == elem {
			return list
		}
	}
	return append(list, elem)
}

func Remove[T comparable](list []T, elem T) []T {
	for i, e := range list {
		if e == elem {
			return RemoveAt(list, i)
		}
	}
	return list
}

func RemoveAt[T any](list []T, index int) []T {
	list[index] = list[len(list)-1]
	return list[:len(list)-1]
}
