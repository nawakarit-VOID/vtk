// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// qualityTags คือคำที่มักปนอยู่ในชื่อไฟล์วิดีโอแต่ไม่ใช่ส่วนหนึ่งของชื่อเรื่องจริง
// (ไม่รวมคำเกี่ยวกับซีซั่น เพราะเลขซีซั่นถูกดึงออกไปจัดการแยกต่างหากใน extractSeason)
var qualityTags = []string{
	"1080p", "720p", "480p", "2160p", "4k",
	"x264", "x265", "h264", "h265", "hevc",
	"bluray", "blu-ray", "bdrip", "webrip", "web-dl", "webdl",
	"dvdrip", "hdtv", "aac", "flac", "dts",
	"10bit", "8bit", "dual audio", "dualaudio",
	"repack", "proper", "uncensored", "uncut",
	"ตอน", "ตอนที่", "ที่", "พากย์ไทย", "ดูหนัง", "เต็มเรื่อง",
	"ดูซีรี่ย์",
}

var bracketRe = regexp.MustCompile(`[\[\(\{][^\]\)\}]*[\]\)\}]`)
var seasonEpRe = regexp.MustCompile(`s\d{1,2}e\d{1,4}`)
var epTokenRe = regexp.MustCompile(`\bep?\.?[\s._-]?\d{1,4}\b`)
var separatorRe = regexp.MustCompile(`[._\-]+`)
var loneNumberRe = regexp.MustCompile(`\b\d{1,4}\b`)
var multiSpaceRe = regexp.MustCompile(`\s+`)

// seasonThRe จับ "ซีซั่น 2" (ไม่ใช้ \b เพราะ regexp ของ Go นับขอบเขตคำแบบ ASCII
// ไม่รู้จักตัวอักษรไทยเป็น word character ทำให้ \b ใช้กับข้อความไทยไม่ได้ผลตามที่ต้องการ)
var seasonThRe = regexp.MustCompile(`ซีซั่น\.?\s*(\d{1,2})`)

// seasonEnRe จับ "season 2" หรือ "ss2" / "ss 2"
var seasonEnRe = regexp.MustCompile(`(?i)\b(?:season|ss)\.?\s*(\d{1,2})\b`)

// extractSeason ดึงเลขซีซั่นออกจากชื่อไฟล์ (ถ้ามี) แล้วคืนชื่อที่ตัดคำระบุซีซั่นออกแล้ว + เลขซีซั่น
// คืนเลขซีซั่น 0 หมายถึงไม่พบ/ไม่ได้ระบุซีซั่น
func extractSeason(name string) (string, int) {
	if m := seasonThRe.FindStringSubmatch(name); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return seasonThRe.ReplaceAllString(name, " "), n
		}
	}
	if m := seasonEnRe.FindStringSubmatch(name); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return seasonEnRe.ReplaceAllString(name, " "), n
		}
	}
	return name, 0
}

// normalizeTitle พยายามดึง "ชื่อเรื่องจริง" ออกจากชื่อไฟล์ โดยตัดเลขตอน/แท็กคุณภาพ/วงเล็บ/เลขซีซั่นออก
// คืนค่าเป็น (ชื่อเรื่องล้วน ๆ ตัวพิมพ์เล็ก, เลขซีซั่นที่เจอ) ใช้สำหรับเทียบความคล้ายเท่านั้น (ไม่ใช้แสดงผลตรง ๆ)
func normalizeTitle(fileName string) (string, int) {
	name := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	name = strings.ToLower(name)

	// ดึงเลขซีซั่นออกไปเก็บต่างหากก่อน ไม่ให้โดนลบทิ้งปนกับเลขตอนทีหลัง
	name, season := extractSeason(name)

	name = bracketRe.ReplaceAllString(name, " ")
	name = seasonEpRe.ReplaceAllString(name, " ")
	name = epTokenRe.ReplaceAllString(name, " ")

	for _, tag := range qualityTags {
		name = strings.ReplaceAll(name, tag, " ")
	}

	name = separatorRe.ReplaceAllString(name, " ")
	name = loneNumberRe.ReplaceAllString(name, " ") // เลขตอนที่หลงเหลืออยู่เดี่ยว ๆ
	name = strings.TrimSpace(multiSpaceRe.ReplaceAllString(name, " "))

	return name, season
}

// toDisplayTitle แปลงชื่อที่ normalize แล้วให้อ่านง่ายขึ้น สำหรับใช้ตั้งชื่อโฟลเดอร์
func toDisplayTitle(normalized string) string {
	if normalized == "" {
		return "untitled"
	}
	words := strings.Fields(normalized)
	for i, w := range words {
		r := []rune(w)
		if len(r) > 0 {
			r[0] = unicode.ToUpper(r[0])
		}
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

// levenshtein คำนวณระยะแก้ไขระหว่างสองสตริง (จำนวนตัวอักษรที่ต้องเพิ่ม/ลบ/แก้)
func levenshtein(a, b string) int {
	ra := []rune(a)
	rb := []rune(b)
	la, lb := len(ra), len(rb)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			m := del
			if ins < m {
				m = ins
			}
			if sub < m {
				m = sub
			}
			curr[j] = m
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

// similarEnough ถือว่า "ชื่อคล้ายกัน" ถ้าความคล้าย (1 - ระยะแก้ไข/ความยาวสูงสุด) >= 0.82
func similarEnough(a, b string) bool {
	if a == b {
		return true
	}
	if a == "" || b == "" {
		return false
	}
	dist := levenshtein(a, b)
	maxLen := len([]rune(a))
	if l := len([]rune(b)); l > maxLen {
		maxLen = l
	}
	sim := 1 - float64(dist)/float64(maxLen)
	return sim >= 0.82
}

// GroupEpisodesByName จัดกลุ่ม episode ที่ชื่อไฟล์ (หลังตัดเลขตอน/แท็กออก) คล้ายกันเข้าด้วยกัน
// ไฟล์จะถูกจัดกลุ่มเดียวกันได้ก็ต่อเมื่อ "ชื่อเรื่องคล้ายกัน" และ "เลขซีซั่นตรงกันเป๊ะ" เท่านั้น
// (ไม่ fuzzy เลขซีซั่น เพื่อไม่ให้ Season 1 กับ Season 2 ถูกรวมกันผิด ๆ)
// คืนค่าเป็น map[ชื่อโฟลเดอร์ที่เสนอ]episode ในกลุ่มนั้น
func GroupEpisodesByName(episodes []*Episode) map[string][]*Episode {
	type cluster struct {
		titleKey string
		season   int
		eps      []*Episode
	}
	var clusters []*cluster
	for _, ep := range episodes {
		titleKey, season := normalizeTitle(ep.FileName)
		if titleKey == "" {
			continue
		}
		var target *cluster
		for _, c := range clusters {
			if c.season == season && similarEnough(c.titleKey, titleKey) {
				target = c
				break
			}
		}
		if target == nil {
			clusters = append(clusters, &cluster{titleKey: titleKey, season: season, eps: []*Episode{ep}})
		} else {
			target.eps = append(target.eps, ep)
		}
	}

	result := map[string][]*Episode{}
	for _, c := range clusters {
		display := toDisplayTitle(c.titleKey)
		if c.season > 0 {
			display = fmt.Sprintf("%s Season %d", display, c.season)
		}
		result[display] = append(result[display], c.eps...)
	}
	return result
}

// GroupProposal คือกลุ่มไฟล์ที่เสนอให้ย้ายไปรวมอยู่ในโฟลเดอร์เดียวกัน
type GroupProposal struct {
	FolderName string
	Episodes   []*Episode
}

// BuildProposals คืนเฉพาะกลุ่มที่มีไฟล์ตั้งแต่ 2 ไฟล์ขึ้นไป (กลุ่มเดี่ยวไม่มีประโยชน์ที่จะย้าย)
func BuildProposals(episodes []*Episode) []GroupProposal {
	groups := GroupEpisodesByName(episodes)
	var proposals []GroupProposal
	for name, eps := range groups {
		if len(eps) < 2 {
			continue
		}
		sort.Slice(eps, func(i, j int) bool { return eps[i].EpisodeNumber < eps[j].EpisodeNumber })
		proposals = append(proposals, GroupProposal{FolderName: name, Episodes: eps})
	}
	sort.Slice(proposals, func(i, j int) bool { return proposals[i].FolderName < proposals[j].FolderName })
	return proposals
}

var invalidFolderCharsRe = regexp.MustCompile(`[\\/:*?"<>|]`)

// sanitizeFolderName ตัดอักขระที่ใช้เป็นชื่อโฟลเดอร์ไม่ได้ออก
func sanitizeFolderName(name string) string {
	clean := invalidFolderCharsRe.ReplaceAllString(name, "")
	clean = strings.TrimSpace(clean)
	if clean == "" {
		clean = "untitled"
	}
	return clean
}
