// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// doubleTapWrapper ห่อ content ใดก็ได้ให้ดับเบิลคลิกได้ (implement fyne.DoubleTappable)
// ใช้กับแถวตอนในหน้าตอน เพื่อดับเบิลคลิกที่แถวแล้วเล่นไฟล์นั้นได้ทันที โดยปุ่มต่าง ๆ
// ที่อยู่ข้างในแถว (เล่น/แก้ชื่อ/ลบ) ยังกดทำงานของตัวเองได้ตามปกติ ไม่ชนกัน
type doubleTapWrapper struct {
	widget.BaseWidget
	content        fyne.CanvasObject
	onDoubleTapped func()
}

func newDoubleTapWrapper(content fyne.CanvasObject, onDoubleTapped func()) *doubleTapWrapper {
	w := &doubleTapWrapper{content: content, onDoubleTapped: onDoubleTapped}
	w.ExtendBaseWidget(w)
	return w
}

func (w *doubleTapWrapper) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(w.content)
}

// DoubleTapped implement fyne.DoubleTappable
func (w *doubleTapWrapper) DoubleTapped(_ *fyne.PointEvent) {
	if w.onDoubleTapped != nil {
		w.onDoubleTapped()
	}
}
