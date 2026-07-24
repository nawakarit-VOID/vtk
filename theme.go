// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type MyTheme struct{}

var (
	// --- PRIMARY COLOR (#BE7C00 = NRGBA{190, 124, 0, 255}) ---
	primaryColor = color.NRGBA{190, 124, 0, 255} // #BE7C00 ส้มทอง/น้ำตาลทองหลัก
	primarySoft  = color.NRGBA{190, 124, 0, 100} // เงาจางๆ ของสีหลัก
	focusTone    = color.NRGBA{190, 124, 0, 60}  // สีขอบไฮไลต์ขณะโฟกัส

	// --- DARK THEME (โทนน้ำตาลเข้มเอสเปรสโซ ตัดกับทอง) ---
	bgDark      = color.NRGBA{32, 26, 20, 255}   // พื้นหลังสีเอสเปรสโซเข้ม
	surfaceDark = color.NRGBA{48, 39, 30, 255}   // ปุ่ม/การ์ด สีไม้เข้ม
	hoverDark   = color.NRGBA{255, 255, 255, 18} // โฮเวอร์
	inputDark   = color.NRGBA{255, 255, 255, 12} // ช่องกรอกข้อมูล
	menuDark    = color.NRGBA{56, 45, 35, 255}   // พื้นหลังเมนู
	overlayDark = color.NRGBA{24, 19, 14, 240}   // ป๊อบอัพ overlay

	// --- LIGHT THEME (โทนครีมทอง / เบจอุ่น) ---
	bgLight      = color.NRGBA{250, 247, 240, 255} // พื้นหลังครีมสว่างนวล
	surfaceLight = color.NRGBA{255, 255, 255, 255} // การ์ด/ปุ่ม สีขาวสะอาด
	hoverLight   = color.NRGBA{190, 124, 0, 20}    // โฮเวอร์โทนส้มทองจางๆ
	inputLight   = color.NRGBA{190, 124, 0, 12}    // ช่องกรอกข้อมูล
	menuLight    = color.NRGBA{255, 255, 255, 255} // เมนู
	overlayLight = color.NRGBA{242, 237, 226, 245} // ป๊อบอัพ overlay

	// --- TEXT & NEUTRAL ---
	textLight    = color.NRGBA{250, 246, 240, 255} // ข้อความสว่าง (สำหรับ Dark Mode)
	textDark     = color.NRGBA{42, 33, 24, 255}    // ข้อความเข้ม (สำหรับ Light Mode)
	disabledTone = color.NRGBA{160, 146, 132, 255} // สถานะ Disabled

	// --- STATUS COLORS (ปรับโทนอุ่นให้เข้ากับ #BE7C00) ---
	errorRed     = color.NRGBA{198, 62, 53, 255}  // แดงอิฐ
	successGreen = color.NRGBA{78, 138, 70, 255}  // เขียวใบชา
	warningAmber = color.NRGBA{220, 145, 15, 255} // เหลืองมัสตาร์ดเข้ม
)

func (m MyTheme) Color(name fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if v == theme.VariantDark {
		switch name {
		case theme.ColorNameBackground:
			return bgDark
		case theme.ColorNameForeground:
			return textLight
		case theme.ColorNameButton:
			return surfaceDark
		case theme.ColorNamePressed:
			return primaryColor
		case theme.ColorNameHover:
			return hoverDark
		case theme.ColorNameDisabledButton:
			return surfaceDark
		case theme.ColorNameDisabled:
			return disabledTone
		case theme.ColorNameFocus:
			return focusTone
		case theme.ColorNamePrimary:
			return primaryColor
		case theme.ColorNameInputBackground:
			return inputDark
		case theme.ColorNamePlaceHolder:
			return color.NRGBA{180, 165, 150, 255}
		case theme.ColorNameMenuBackground:
			return menuDark
		case theme.ColorNameOverlayBackground:
			return overlayDark
		case theme.ColorNameShadow:
			return primarySoft
		case theme.ColorNameError:
			return errorRed
		case theme.ColorNameSuccess:
			return successGreen
		case theme.ColorNameWarning:
			return warningAmber
		}
	} else {
		switch name {
		case theme.ColorNameBackground:
			return bgLight
		case theme.ColorNameForeground:
			return textDark
		case theme.ColorNameButton:
			return color.NRGBA{190, 124, 0, 18}
		case theme.ColorNamePressed:
			return primaryColor
		case theme.ColorNameHover:
			return hoverLight
		case theme.ColorNameDisabledButton:
			return color.NRGBA{190, 175, 160, 90}
		case theme.ColorNameDisabled:
			return disabledTone
		case theme.ColorNameFocus:
			return focusTone
		case theme.ColorNamePrimary:
			return primaryColor
		case theme.ColorNameInputBackground:
			return inputLight
		case theme.ColorNamePlaceHolder:
			return color.NRGBA{140, 125, 110, 255}
		case theme.ColorNameMenuBackground:
			return menuLight
		case theme.ColorNameOverlayBackground:
			return overlayLight
		case theme.ColorNameShadow:
			return primarySoft
		case theme.ColorNameError:
			return errorRed
		case theme.ColorNameSuccess:
			return successGreen
		case theme.ColorNameWarning:
			return warningAmber
		}
	}
	return theme.DefaultTheme().Color(name, v)
}

// ต้องมีครบ
func (m MyTheme) Font(s fyne.TextStyle) fyne.Resource {
	return myFont
	//return theme.DefaultTheme().Font(s)
}
func (m MyTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(n)
}
func (m MyTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {

	// 📏 spacing / ระยะ
	case theme.SizeNamePadding: // → ระยะห่างทั่วไป (margin/padding ของ widget)
		return 4
	case theme.SizeNameSeparatorThickness: // → ความหนาเส้นคั่น
		return 1

	// 🖼️ ไอคอน / scrollbar
	case theme.SizeNameInlineIcon: // → ขนาด icon ในปุ่ม/ข้อความ /dialog
		return 19

	case theme.SizeNameScrollBar: // → ความกว้าง scrollbar ปกติ
		return 12
	case theme.SizeNameScrollBarSmall: // → scrollbar แบบเล็ก
		return 3

	// 🔤 ขนาดตัวอักษร
	case theme.SizeNameText: // → ข้อความปกติ
		return 14
	case theme.SizeNameHeadingText: // → หัวข้อใหญ่
		return 20
	case theme.SizeNameSubHeadingText: // → หัวข้อรอง
		return 16
	case theme.SizeNameCaptionText: // → ตัวเล็ก (caption/คำอธิบาย)
		return 12

	// 🧾 input
	case theme.SizeNameInputBorder: // → ความหนาขอบ input
		return 1
	}
	return theme.DefaultTheme().Size(name)
}
