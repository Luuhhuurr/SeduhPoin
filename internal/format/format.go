// Package format berisi fungsi bantu untuk memformat angka, uang, dan tanggal
// agar tampilan di template konsisten.
//
// Seluruh nominal uang dan poin di proyek ini bertipe int64 (dilarang float
// untuk nominal), sehingga fungsi format di sini juga menerima int64.
package format

import (
	"fmt"
	"time"
)

// ZonaWIB adalah zona waktu seluruh aplikasi (BR: semua timestamp WIB).
const ZonaWIB = "Asia/Jakarta"

// WIB mengembalikan waktu saat ini pada zona Asia/Jakarta.
//
// Nilai time.LoadLocation sudah di-cache oleh paket time itu sendiri, jadi
// memanggilnya berkali-kali tetap murah.
func WIB() time.Time {
	loc, err := time.LoadLocation(ZonaWIB)
	if err != nil {
		return time.Now()
	}
	return time.Now().In(loc)
}

// HariIni mengembalikan tanggal (00:00:00) hari ini pada zona WIB.
func HariIni() time.Time {
	t := WIB()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// Rupiah memformat nominal menjadi "Rp 145.000".
func Rupiah(n int64) string {
	return "Rp " + Angka(n)
}

// Angka memformat bilangan bulat dengan pemisah ribuan titik.
//
// Nilai negatif ditangani dengan tanda minus di depan, dan angka diubah ke
// string terlebih dahulu agar pemisah ribuan bisa disisipkan setiap tiga digit.
func Angka(n int64) string {
	negatif := n < 0
	if negatif {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	hasil := joinTitik(parts)
	if negatif {
		return "-" + hasil
	}
	return hasil
}

func joinTitik(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "."
		}
		out += p
	}
	return out
}

// Tanggal memformat waktu menjadi "02 Nov 2026 14:30".
//
// t.In(LokasiWIB()) memastikan waktu yang ditampilkan selalu dalam zona WIB,
// apa pun zona waktu tempat server dijalankan.
func Tanggal(t time.Time) string {
	return t.In(LokasiWIB()).Format("02 Jan 2006 15:04")
}

// TanggalSingkat memformat waktu menjadi "02 Nov 2026".
func TanggalSingkat(t time.Time) string {
	return t.In(LokasiWIB()).Format("02 Jan 2006")
}

// Bulan memformat waktu menjadi "November 2026".
func Bulan(t time.Time) string {
	return t.In(LokasiWIB()).Format("January 2006")
}

// KeyBulan mengembalikan kunci pengelompokan bulanan "2026-11".
//
// Kunci ini dipakai untuk GROUP BY per bulan pada laporan FR-19 dan FR-20.
func KeyBulan(t time.Time) string {
	return t.In(LokasiWIB()).Format("2006-01")
}

// LabelBulan mengembalikan nama bulan singkat "Nov 2026" dari kunci "2026-11".
func LabelBulan(key string) string {
	t, err := time.ParseInLocation("2006-01", key, LokasiWIB())
	if err != nil {
		return key
	}
	return t.Format("Jan 2006")
}

// LokasiWIB mengembalikan *time.Location untuk zona Asia/Jakarta.
//
// Windows sering tidak menyediakan basis data zona waktu sistem, sehingga
// main.go meng-import _ "time/tzdata" agar zona ini selalu tersedia.
func LokasiWIB() *time.Location {
	loc, err := time.LoadLocation(ZonaWIB)
	if err != nil {
		return time.UTC
	}
	return loc
}

// namaBulanIndonesia berisi nama bulan dalam Bahasa Indonesia.
var namaBulanIndonesia = []string{
	"Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember",
}

// LabelBulanPanjang mengembalikan nama bulan Bahasa Indonesia dari kunci
// "2026-11", misalnya "November 2026".
func LabelBulanPanjang(key string) string {
	t, err := time.ParseInLocation("2006-01", key, LokasiWIB())
	if err != nil {
		return key
	}
	return namaBulanIndonesia[int(t.Month())-1] + " " + t.Format("2006")
}
