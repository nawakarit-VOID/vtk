// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var videoExtensions = map[string]bool{
	".mp4": true, ".mkv": true, ".avi": true, ".mov": true,
	".wmv": true, ".flv": true, ".webm": true, ".m4v": true,
	".ts": true, ".mpg": true, ".mpeg": true,
}

// ลำดับ pattern สำหรับดึงเลขตอนจากชื่อไฟล์ (ลองตามลำดับ ตัวแรกที่ match คือคำตอบ)
var epPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)S\d{1,2}E(\d{1,4})`),      // S01E12
	regexp.MustCompile(`(?i)EP?[\s._-]?(\d{1,4})`),    // EP12, E12, ep 12
	regexp.MustCompile(`[\[\(]\s*(\d{1,4})\s*[\]\)]`), // [12] หรือ (12)
	regexp.MustCompile(`(\d{1,4})\s*$`),               // เลขท้ายชื่อไฟล์ (ไม่รวมนามสกุล)
	regexp.MustCompile(`(\d{1,4})`),                   // fallback: เลขตัวแรกที่เจอ
}

func ExtractEpisodeNumber(fileName string) int {
	name := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	for _, re := range epPatterns {
		matches := re.FindStringSubmatch(name)
		if matches != nil {
			numStr := matches[len(matches)-1]
			if n, err := strconv.Atoi(numStr); err == nil {
				return n
			}
		}
	}
	return 0
}

func isVideoFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return videoExtensions[ext]
}

// ScanFolder สแกน rootPath: แต่ละโฟลเดอร์ย่อยกลายเป็น 1 ซีรีส์
// ไฟล์วิดีโอที่อยู่ตรง rootPath โดยตรง (ไม่ได้อยู่ในโฟลเดอร์ย่อย) จะถูกจัดเป็น "โฟลเดอร์แม่"
// และแสดงไว้บนสุดของผลลัพธ์เสมอ แยกออกจากโฟลเดอร์ย่อยอื่น ๆ ที่เรียงตามตัวอักษร
func ScanFolder(rootPath string) ([]*Series, error) {
	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return nil, err
	}

	var subSeries []*Series
	var rootFiles []*Episode

	for _, entry := range entries {
		fullPath := filepath.Join(rootPath, entry.Name())
		if entry.IsDir() {
			s, err := scanSeriesFolder(entry.Name(), fullPath)
			if err != nil {
				continue
			}
			if len(s.Episodes) > 0 {
				subSeries = append(subSeries, s)
			}
		} else if isVideoFile(entry.Name()) {
			rootFiles = append(rootFiles, &Episode{
				FilePath:      fullPath,
				FileName:      entry.Name(),
				EpisodeNumber: ExtractEpisodeNumber(entry.Name()),
				Exists:        true,
			})
		}
	}

	// เรียงโฟลเดอร์ย่อยตามตัวอักษร (ไม่รวมโฟลเดอร์แม่)
	sort.Slice(subSeries, func(i, j int) bool { return subSeries[i].Name < subSeries[j].Name })

	var seriesList []*Series

	// โฟลเดอร์แม่ (ไฟล์หลวม ๆ ตรง root) มาก่อนเสมอ ถ้ามี
	if len(rootFiles) > 0 {
		sort.Slice(rootFiles, func(i, j int) bool { return rootFiles[i].EpisodeNumber < rootFiles[j].EpisodeNumber })
		seriesList = append(seriesList, &Series{
			Name:     filepath.Base(rootPath),
			RootPath: rootPath,
			Episodes: rootFiles,
		})
	}

	seriesList = append(seriesList, subSeries...)

	return seriesList, nil
}

func scanSeriesFolder(name, path string) (*Series, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	s := &Series{Name: name, RootPath: path}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if isVideoFile(entry.Name()) {
			s.Episodes = append(s.Episodes, &Episode{
				FilePath:      filepath.Join(path, entry.Name()),
				FileName:      entry.Name(),
				EpisodeNumber: ExtractEpisodeNumber(entry.Name()),
				Exists:        true,
			})
		}
	}
	sort.Slice(s.Episodes, func(i, j int) bool { return s.Episodes[i].EpisodeNumber < s.Episodes[j].EpisodeNumber })
	return s, nil
}

// MergeScan รวมผลสแกนใหม่เข้ากับข้อมูลเดิม โดย:
//   - ไฟล์ที่ยังเจอ -> อัปเดตข้อมูล และคงสถานะ Watched เดิมไว้
//   - ไฟล์ใหม่ที่ไม่เคยเจอ -> เพิ่มเป็นตอนใหม่ (ยังไม่ดู)
//   - ไฟล์ที่เคยมีแต่ตอนนี้หายไป -> คงไว้ในลิสต์ (เก็บประวัติ) แต่ตั้ง Exists = false
func MergeScan(lib *Library, scanned []*Series, rootPath string) {
	existingBySeries := map[string]*Series{}
	for _, s := range lib.SeriesList {
		existingBySeries[s.Name] = s
	}

	scannedBySeries := map[string]*Series{}
	for _, s := range scanned {
		scannedBySeries[s.Name] = s
	}

	for _, scannedSeries := range scanned {
		existing, found := existingBySeries[scannedSeries.Name]
		if !found {
			lib.SeriesList = append(lib.SeriesList, scannedSeries)
			existingBySeries[scannedSeries.Name] = scannedSeries
			continue
		}
		existing.RootPath = scannedSeries.RootPath

		existingByPath := map[string]*Episode{}
		for _, e := range existing.Episodes {
			existingByPath[e.FilePath] = e
		}

		newEpByPath := map[string]bool{}
		for _, ne := range scannedSeries.Episodes {
			newEpByPath[ne.FilePath] = true
			if oe, ok := existingByPath[ne.FilePath]; ok {
				oe.Exists = true
				oe.FileName = ne.FileName
				oe.EpisodeNumber = ne.EpisodeNumber
			} else {
				existing.Episodes = append(existing.Episodes, ne)
			}
		}

		for _, oe := range existing.Episodes {
			if !newEpByPath[oe.FilePath] {
				oe.Exists = false
			}
		}

		sort.Slice(existing.Episodes, func(i, j int) bool {
			return existing.Episodes[i].EpisodeNumber < existing.Episodes[j].EpisodeNumber
		})
	}

	// ซีรีส์เดิมที่อยู่ในโฟลเดอร์ที่สแกนรอบนี้ แต่ไม่เจอเลยในรอบนี้ -> ไฟล์ในนั้นถือว่าถูกลบหมด
	for _, existing := range lib.SeriesList {
		if _, found := scannedBySeries[existing.Name]; !found {
			if strings.HasPrefix(existing.RootPath, rootPath) {
				for _, e := range existing.Episodes {
					if strings.HasPrefix(e.FilePath, rootPath) {
						e.Exists = false
					}
				}
			}
		}
	}
}
