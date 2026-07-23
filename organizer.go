// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
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

// qualityTagRes คือ qualityTags ที่คอมไพล์เป็น regex case-insensitive ไว้ล่วงหน้า
// ใช้สำหรับตอนตั้งชื่อโฟลเดอร์ (buildDisplayName) ซึ่งต้องคงตัวพิมพ์เล็ก/ใหญ่เดิมของชื่อเรื่องไว้
// (ต่างจาก normalizeTitle ที่แปลงเป็นตัวพิมพ์เล็กหมดก่อนเพราะใช้เทียบความคล้ายเท่านั้น)
var qualityTagRes = compileQualityTagRes()

func compileQualityTagRes() []*regexp.Regexp {
	res := make([]*regexp.Regexp, 0, len(qualityTags))
	for _, t := range qualityTags {
		res = append(res, regexp.MustCompile(`(?i)`+regexp.QuoteMeta(t)))
	}
	return res
}

var bracketRe = regexp.MustCompile(`[\[\(\{][^\]\)\}]*[\]\)\}]`)
var seasonEpRe = regexp.MustCompile(`(?i)s\d{1,2}e\d{1,4}`)         // S01E12 แบบโค้ดรวม (ไม่ใช่ตัวหนังสือ)
var epTokenRe = regexp.MustCompile(`(?i)\bep?\.?[\s._-]?\d{1,4}\b`) // EP12 / E12
var thEpTokenRe = regexp.MustCompile(`(?:ตอนที่|ตอน)\.?\s*\d{1,4}`) // ตอนที่ 6 / ตอน 06 (ตัดคำ+เลขเป็นก้อนเดียว
// เหมือน epTokenRe ของฝั่งอังกฤษ จะได้ไม่ต้องพึ่งการจับคู่เลขตอนแบบเป๊ะทีหลัง ซึ่งพังง่ายเวลาเลขมีศูนย์นำหน้า เช่น "06")
var separatorRe = regexp.MustCompile(`[._\-]+`)
var loneNumberRe = regexp.MustCompile(`\b\d{1,4}\b`)
var multiSpaceRe = regexp.MustCompile(`\s+`)

// seasonPatterns คือ "คำที่ใช้บอกเลขซีซั่น" ทั้งหมดที่รู้จัก แต่ละตัวต้องมี capture group เดียว
// สำหรับเลขซีซั่น (1-2 หลัก เพื่อไม่ให้ชนกับปี ค.ศ. 4 หลักอย่าง "หนังปี 2019")
//
// ใช้เฉพาะตอน "จับกลุ่มไฟล์" (หาว่าไฟล์ไหนเป็นซีซั่นเดียวกัน) เท่านั้น
// ไม่ได้เอาไปตัดคำออกจากชื่อโฟลเดอร์จริง (ดู buildDisplayName ที่คงคำพวกนี้ไว้)
//
// ในอนาคตถ้าเจอคำใหม่ที่ใช้บอกซีซั่น (เช่น "ภาค", "part") แค่เพิ่ม regex อีกบรรทัดเข้ามาใน list นี้
var seasonPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?:ซีซั่น|ปี)\.?\s*(\d{1,2})`),
	regexp.MustCompile(`(?i)\b(?:season|ss)\.?\s*(\d{1,2})\b`),
}

// extractSeason ดึงเลขซีซั่นออกจากชื่อไฟล์ (ถ้ามี) แล้วคืนชื่อที่ตัดคำระบุซีซั่นออกแล้ว + เลขซีซั่น
// วนลูปใช้ทุก pattern ใน seasonPatterns กับชื่อไฟล์เสมอ (ไม่ return ทันทีที่เจอตัวแรก)
// เพื่อให้รองรับกรณีมีหลายคำ/หลายภาษาปนกันในชื่อเดียวกัน
func extractSeason(name string) (string, int) {
	season := 0
	for _, re := range seasonPatterns {
		if m := re.FindStringSubmatch(name); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil {
				season = n
			}
			name = re.ReplaceAllString(name, " ")
		}
	}
	return name, season
}

// normalizeTitle ใช้สำหรับ "จับกลุ่ม" เท่านั้น: ตัดเลขตอน/แท็กคุณภาพ/วงเล็บ/คำบอกซีซั่นออกให้เหลือแก่นชื่อเรื่อง
// คืนค่าเป็น (ชื่อเรื่องล้วน ๆ ตัวพิมพ์เล็ก, เลขซีซั่นที่เจอ)
func normalizeTitle(fileName string) (string, int) {
	name := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	name = strings.ToLower(name)

	name, season := extractSeason(name)

	name = bracketRe.ReplaceAllString(name, " ")
	name = seasonEpRe.ReplaceAllString(name, " ")
	name = epTokenRe.ReplaceAllString(name, " ")
	name = thEpTokenRe.ReplaceAllString(name, " ")

	for _, tag := range qualityTags {
		name = strings.ReplaceAll(name, tag, " ")
	}

	name = separatorRe.ReplaceAllString(name, " ")
	name = loneNumberRe.ReplaceAllString(name, " ") // เลขตอน/เลขซีซั่นที่หลงเหลืออยู่เดี่ยว ๆ
	name = strings.TrimSpace(multiSpaceRe.ReplaceAllString(name, " "))

	return name, season
}

// stripTrailingEpisodeNumber ตัดเลขตอน (ที่รู้ค่าแน่นอนอยู่แล้วจากตอน scan) ออกจากท้ายชื่อ
// ตัดเฉพาะตอนที่มันอยู่ท้ายสุดของชื่อ (หลัง trim) เท่านั้น เพื่อไม่ไปพลาดตัดเลขซีซั่นที่อยู่กลางชื่อ
// เช่น "Rick and Morty Season 9 ริค แอนด์ มอร์ตี้ ปี 9   6" -> ตัด "6" ท้ายสุดออก เหลือเลขซีซั่น 9 ไว้ครบ
// รองรับเลขตอนที่เติมศูนย์นำหน้าด้วย เช่น epNum=6 ต้องตัด "06" หรือ "006" ที่อยู่ท้ายชื่อได้เช่นกัน
// (ถ้าใช้ \b6\b ตรง ๆ จะไม่เจอ เพราะ "0" กับ "6" ใน "06" เป็นตัวอักษรกลุ่มเดียวกัน ไม่มีขอบเขตคำคั่นกลาง)
func stripTrailingEpisodeNumber(name string, epNum int) string {
	trimmed := strings.TrimSpace(name)
	if epNum <= 0 {
		return trimmed
	}
	re := regexp.MustCompile(`\b0*` + strconv.Itoa(epNum) + `\b\s*$`)
	trimmed = re.ReplaceAllString(trimmed, "")
	return strings.TrimSpace(trimmed)
}

// buildDisplayName สร้างชื่อสำหรับตั้งเป็นโฟลเดอร์ โดยคงคำเดิมในชื่อไฟล์ไว้ให้มากที่สุด
// (รวมถึงคำว่า Season/ปี ที่บอกซีซั่น) ตัดออกเฉพาะ: นามสกุลไฟล์, วงเล็บ, โค้ด SxxExx,
// แท็ก EP/E ที่เป็นตัวเลขตอน, แท็กคุณภาพ/เทคนิค และเลขตอนของไฟล์นั้น ๆ ที่อยู่ท้ายชื่อ
func buildDisplayName(ep *Episode) string {
	name := strings.TrimSuffix(ep.FileName, filepath.Ext(ep.FileName))

	name = bracketRe.ReplaceAllString(name, " ")
	name = seasonEpRe.ReplaceAllString(name, " ")

	// ถ้าเจอคำมาร์คเกอร์ตอน ("ตอนที่"/"EP") ชัดเจน ถือว่าตัดเลขตอนไปพร้อมกันแล้ว
	// ไม่ต้องพึ่ง fallback อีก เพราะถ้ารันซ้ำอาจไปตัดเลขซีซั่นที่เหลืออยู่ท้ายชื่อผิด ๆ
	// ในกรณีที่เลขตอนบังเอิญตรงกับเลขซีซั่น (เช่น ซีซั่น 9 ตอนที่ 9)
	hasExplicitMarker := epTokenRe.MatchString(name) || thEpTokenRe.MatchString(name)
	name = epTokenRe.ReplaceAllString(name, " ")
	name = thEpTokenRe.ReplaceAllString(name, " ")

	for _, re := range qualityTagRes {
		name = re.ReplaceAllString(name, " ")
	}

	name = separatorRe.ReplaceAllString(name, " ")
	name = strings.TrimSpace(multiSpaceRe.ReplaceAllString(name, " "))

	if !hasExplicitMarker {
		// fallback: เผื่อไม่มีคำว่า "ตอน"/"EP" นำหน้าเลย มีแต่เลขลอย ๆ ท้ายชื่อ
		name = stripTrailingEpisodeNumber(name, ep.EpisodeNumber)
	}

	if name == "" {
		return "untitled"
	}
	return name
}

// mostCommonDisplayName เลือกชื่อโฟลเดอร์ตัวแทนของกลุ่ม โดยใช้ชื่อที่ซ้ำกันบ่อยที่สุด
// (ปกติทุก episode ในกลุ่มเดียวกันควรได้ display name เดียวกันอยู่แล้ว
// เพราะต่างกันแค่เลขตอนซึ่งถูกตัดออกไปแล้ว)
func mostCommonDisplayName(episodes []*Episode) string {
	counts := map[string]int{}
	var order []string
	for _, ep := range episodes {
		d := buildDisplayName(ep)
		if _, ok := counts[d]; !ok {
			order = append(order, d)
		}
		counts[d]++
	}
	best := order[0]
	for _, d := range order {
		if counts[d] > counts[best] {
			best = d
		}
	}
	return best
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

// GroupProposal คือกลุ่มไฟล์ที่เสนอให้ย้ายไปรวมอยู่ในโฟลเดอร์เดียวกัน
// ถ้า ExistingSeries ไม่ใช่ nil แปลว่ามีโฟลเดอร์ที่ชื่อคล้ายกันอยู่แล้ว ให้ย้ายเข้าโฟลเดอร์นั้นแทนที่จะสร้างใหม่
type GroupProposal struct {
	FolderName     string
	Episodes       []*Episode
	ExistingSeries *Series
}

// siblingKey คือ title+season ที่ normalize แล้วของโฟลเดอร์ย่อยที่มีอยู่แล้ว ใช้เทียบกับไฟล์หลวม ๆ
type siblingKey struct {
	titleKey string
	season   int
	series   *Series
}

// findMatchingSibling หาโฟลเดอร์ย่อยที่มีอยู่แล้วซึ่ง "ชื่อเรื่องคล้ายกัน AND เลขซีซั่นตรงกันเป๊ะ" กับ titleKey/season ที่ให้มา
func findMatchingSibling(titleKey string, season int, siblingKeys []siblingKey) *Series {
	for _, sk := range siblingKeys {
		if sk.season == season && similarEnough(sk.titleKey, titleKey) {
			return sk.series
		}
	}
	return nil
}

// BuildProposals จัดกลุ่มไฟล์ที่ชื่อคล้ายกัน แล้วเสนอปลายทางให้:
//   - ถ้ากลุ่มนั้นตรงกับโฟลเดอร์ย่อยที่มีอยู่แล้ว (siblings) -> เสนอย้ายเข้าโฟลเดอร์เดิม (แม้จะมีไฟล์เดียวในกลุ่มก็ตาม)
//   - ถ้าไม่ตรงกับโฟลเดอร์ไหนเลย และมีไฟล์ตั้งแต่ 2 ไฟล์ขึ้นไปในกลุ่ม -> เสนอสร้างโฟลเดอร์ใหม่
//   - กลุ่มที่มีไฟล์เดียวและไม่ตรงกับโฟลเดอร์ไหนเลย -> ข้าม (ไม่มีประโยชน์ที่จะย้าย)
func BuildProposals(episodes []*Episode, siblings []*Series) []GroupProposal {
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

	var siblingKeys []siblingKey
	for _, sib := range siblings {
		tk, sn := normalizeTitle(sib.Name)
		if tk == "" {
			continue
		}
		siblingKeys = append(siblingKeys, siblingKey{titleKey: tk, season: sn, series: sib})
	}

	var proposals []GroupProposal
	for _, c := range clusters {
		matched := findMatchingSibling(c.titleKey, c.season, siblingKeys)

		if matched == nil && len(c.eps) < 2 {
			continue // กลุ่มเดี่ยว ไม่มีโฟลเดอร์เดิมให้ย้ายเข้า ไม่มีประโยชน์ที่จะย้าย
		}

		sort.Slice(c.eps, func(i, j int) bool { return c.eps[i].EpisodeNumber < c.eps[j].EpisodeNumber })

		folderName := mostCommonDisplayName(c.eps)
		if matched != nil {
			folderName = matched.Name // ใช้ชื่อโฟลเดอร์เดิมที่มีอยู่แล้วโชว์ในพรีวิว
		}

		proposals = append(proposals, GroupProposal{
			FolderName:     folderName,
			Episodes:       c.eps,
			ExistingSeries: matched,
		})
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

// uniqueDestPath คืน path ปลายทางที่ไม่ชนกับไฟล์ที่มีอยู่แล้วในโฟลเดอร์ dir
// ถ้าชื่อไฟล์ซ้ำ จะเติม " (1)", " (2)", ... ต่อท้าย (ก่อนนามสกุล) ให้อัตโนมัติ
func uniqueDestPath(dir, fileName string) string {
	ext := filepath.Ext(fileName)
	base := strings.TrimSuffix(fileName, ext)

	candidate := filepath.Join(dir, fileName)
	for i := 1; ; i++ {
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
		candidate = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
	}
}
