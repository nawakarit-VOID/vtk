// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import "strings"

// naturalLess เทียบสองสตริงแบบ "natural sort" คือถ้าเจอช่วงตัวเลข จะเทียบเป็นค่าตัวเลขจริง
// ไม่ใช่เทียบทีละตัวอักษรแบบสตริงธรรมดา ทำให้ "ตอนที่ 2" มาก่อน "ตอนที่ 10" ตามที่ควรจะเป็น
// (ถ้าเทียบแบบสตริงธรรมดา "10" จะมาก่อน "2" เพราะ '1' < '2')
func naturalLess(a, b string) bool {
	ia, ib := 0, 0
	la, lb := len(a), len(b)

	for ia < la && ib < lb {
		ca, cb := a[ia], b[ib]

		if isASCIIDigit(ca) && isASCIIDigit(cb) {
			startA := ia
			for ia < la && isASCIIDigit(a[ia]) {
				ia++
			}
			startB := ib
			for ib < lb && isASCIIDigit(b[ib]) {
				ib++
			}

			numA := strings.TrimLeft(a[startA:ia], "0")
			numB := strings.TrimLeft(b[startB:ib], "0")

			if len(numA) != len(numB) {
				return len(numA) < len(numB)
			}
			if numA != numB {
				return numA < numB
			}
			// ค่าตัวเลขเท่ากัน (เช่น "02" กับ "2") ไปเทียบส่วนที่เหลือต่อ
			continue
		}

		if ca != cb {
			return ca < cb
		}
		ia++
		ib++
	}

	return (la - ia) < (lb - ib)
}

func isASCIIDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
