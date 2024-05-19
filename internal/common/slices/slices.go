// SPDX-License-Identifier: Apache-2.0

package slices

import "slices"

func Any[T any](list []T, predicate func(T) bool) bool {
	for _, e := range list {
		if predicate(e) {
			return true
		}
	}
	return false
}

func Filter[T any](list []T, predicate func(T) bool) []T {
	var result []T
	for _, e := range list {
		if predicate(e) {
			result = append(result, e)
		}
	}
	return result
}

func Map[T any, U any](list []T, f func(T) U) []U {
	result := make([]U, 0, len(list))
	for _, e := range list {
		result = append(result, f(e))
	}
	return result
}

func TryMap[T any, U any](list []T, f func(T) (U, error)) ([]U, error) {
	result := make([]U, 0, len(list))
	for _, e := range list {
		r, err := f(e)
		if err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, nil
}

func AppendUnique[T comparable](list []T, elem T) []T {
	for _, e := range list {
		if e == elem {
			return list
		}
	}
	return append(list, elem)
}

func RemoveAll[T comparable](list []T, elem T) []T {
	for i := 0; i < len(list); i++ {
		if list[i] == elem {
			list = slices.Delete(list, i, i+1)
			i--
		}
	}
	return list
}

func SetsEqual[T comparable](a, b []T) bool {
	for _, e := range a {
		if !slices.Contains(b, e) {
			return false
		}
	}

	for _, e := range b {
		if !slices.Contains(a, e) {
			return false
		}
	}

	return true
}

func Deduplicate[T comparable](list []T) []T {
	var result []T
	for _, e := range list {
		if !slices.Contains(result, e) {
			result = append(result, e)
		}
	}
	return result
}

func CountNonNil(vs ...interface{}) int {
	count := 0
	for _, v := range vs {
		if v != nil {
			count++
		}
	}
	return count
}
