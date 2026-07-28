// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// seriesRow คือแถวที่กดเลือกได้ 1 แถวในลิสต์ซีรีส์ทางซ้าย
// ใช้ container.NewVBox ธรรมดาแทน widget.List เพื่อให้แต่ละแถวจองความสูงตามเนื้อหาจริงของตัวเอง
// (widget.List บังคับทุกแถวสูงเท่ากันหมด เพราะเป็น list แบบ virtualized)
type seriesRow struct {
	widget.BaseWidget
	label      *widget.Label
	onTapped   func()
	isSelected bool
}

func newSeriesRow(text string, onTapped func()) *seriesRow {
	r := &seriesRow{
		label:    widget.NewLabel(text),
		onTapped: onTapped,
	}
	r.label.Wrapping = fyne.TextWrapWord
	r.ExtendBaseWidget(r)
	return r
}

func (r *seriesRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.label)
}

// Tapped ทำให้แถวนี้กดเลือกได้ (implement fyne.Tappable)
func (r *seriesRow) Tapped(_ *fyne.PointEvent) {
	if r.onTapped != nil {
		r.onTapped()
	}
}

func (r *seriesRow) SetText(text string) {
	r.label.SetText(text)
}

// SetSelected ปรับสไตล์ให้เห็นว่าแถวนี้ถูกเลือกอยู่ (ตัวหนา/สีเน้น) แทนไฮไลต์แบบ widget.List เดิม
func (r *seriesRow) SetSelected(selected bool) {
	r.isSelected = selected
	if selected {
		r.label.Importance = widget.HighImportance
		r.label.TextStyle = fyne.TextStyle{Bold: true}
	} else {
		r.label.Importance = widget.MediumImportance
		r.label.TextStyle = fyne.TextStyle{}
	}
	r.label.Refresh()
}
