// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// seriesRow คือแถวที่กดเลือกได้ 1 แถวในลิสต์ซีรีส์ทางซ้าย
// ใช้ container.NewVBox ธรรมดาแทน widget.List เพื่อให้แต่ละแถวจองความสูงตามเนื้อหาจริงของตัวเอง
// (widget.List บังคับทุกแถวสูงเท่ากันหมด เพราะเป็น list แบบ virtualized)
type seriesRow struct {
	widget.BaseWidget
	content        *fyne.Container
	label          *widget.Label
	starBtn        *widget.Button
	onTapped       func()
	onDoubleTapped func()
	isSelected     bool
	libIndex       int // ตำแหน่งจริงใน lib.SeriesList (ไม่ใช่ตำแหน่งในลิสต์ที่กรองแล้ว ใช้เทียบ selectedIdx ให้ถูกต้อง)
}

// newSeriesRow สร้างแถวซีรีส์ 1 แถว
//   - showStar: แสดงปุ่มติดดาวไหม (โฟลเดอร์แม่ไม่ต้องมี เพราะอยู่บนสุดอยู่แล้ว)
//   - starred: สถานะติดดาวปัจจุบัน (ใช้ตอน showStar = true เท่านั้น)
//   - onTapped: กดที่แถว 1 ครั้ง (เลือกดูซีรีส์นี้)
//   - onDoubleTapped: ดับเบิลคลิกที่แถว (เล่นซีรีส์นี้ทั้งหมดทันที)
//   - onStarTapped: กดปุ่มดาว (สลับสถานะติดดาว) แยกจาก onTapped ไม่ทำให้แถวถูกเลือกไปด้วย
func newSeriesRow(text string, showStar bool, starred bool, onTapped func(), onDoubleTapped func(), onStarTapped func()) *seriesRow {
	r := &seriesRow{
		label:          widget.NewLabel(text),
		onTapped:       onTapped,
		onDoubleTapped: onDoubleTapped,
	}
	r.label.Wrapping = fyne.TextWrapWord

	if showStar {
		r.starBtn = widget.NewButton(starGlyph(starred), onStarTapped)
		r.starBtn.Importance = widget.LowImportance
		r.content = container.NewBorder(nil, nil, nil, r.starBtn, r.label)
	} else {
		r.content = container.NewBorder(nil, nil, nil, nil, r.label)
	}

	r.ExtendBaseWidget(r)
	return r
}

func starGlyph(starred bool) string {
	if starred {
		return "★"
	}
	return "☆"
}

func (r *seriesRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.content)
}

// Tapped ทำให้แถวนี้กดเลือกได้ (implement fyne.Tappable)
// (ปุ่มดาวข้างในจะรับ tap ของตัวเองไปก่อนอยู่แล้ว ไม่ทำให้ Tapped ของแถวถูกเรียกซ้ำ)
func (r *seriesRow) Tapped(_ *fyne.PointEvent) {
	if r.onTapped != nil {
		r.onTapped()
	}
}

// DoubleTapped ทำให้ดับเบิลคลิกที่แถวนี้ได้ (implement fyne.DoubleTappable)
func (r *seriesRow) DoubleTapped(_ *fyne.PointEvent) {
	if r.onDoubleTapped != nil {
		r.onDoubleTapped()
	}
}

func (r *seriesRow) SetText(text string) {
	r.label.SetText(text)
}

// SetStarred อัปเดตหน้าปุ่มดาวให้ตรงกับสถานะปัจจุบัน (เรียกหลังสลับสถานะ)
func (r *seriesRow) SetStarred(starred bool) {
	if r.starBtn != nil {
		r.starBtn.SetText(starGlyph(starred))
	}
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
