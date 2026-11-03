package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"seduhpoin/internal/format"
)

// redirect mengarahkan pengguna ke URL lain dengan pesan error (opsional).
func redirect(w http.ResponseWriter, r *http.Request, target string, errMsg string) {
	if errMsg != "" {
		target += separatorURL(target) + "err=" + url.QueryEscape(errMsg)
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func separatorURL(target string) string {
	if strings.Contains(target, "?") {
		return "&"
	}
	return "?"
}

// ambilError membaca pesan error dari query string (dipakai setelah redirect).
func ambilError(r *http.Request) string {
	return r.URL.Query().Get("err")
}

// redirectSukses mengarahkan pengguna ke URL lain dengan pesan sukses.
func redirectSukses(w http.ResponseWriter, r *http.Request, target, pesan string) {
	target += separatorURL(target) + "ok=" + url.QueryEscape(pesan)
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// ambilSukses membaca pesan sukses dari query string.
func ambilSukses(r *http.Request) string {
	return r.URL.Query().Get("ok")
}

// idDariPath mengambil parameter id dari path dan memvalidasinya.
func idDariPath(r *http.Request) (int64, error) {
	raw := r.PathValue("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("ID tidak valid: %q", raw)
	}
	return id, nil
}

// parseInt64 mengubah string menjadi int64 dengan nilai default.
func parseInt64(raw string, defaultNilai int64) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultNilai
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return defaultNilai
	}
	return n
}

// parseTanggal mem-parsing tanggal berformat YYYY-MM-DD.
func parseTanggal(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("tanggal tidak boleh kosong")
	}
	return time.ParseInLocation("2006-01-02", raw, format.LokasiWIB())
}

// parseTanggalOpsional mem-parsing tanggal; string kosong menghasilkan waktu zero.
func parseTanggalOpsional(raw string) (time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return time.Time{}, nil
	}
	return parseTanggal(raw)
}

// tanggalDefault mengembalikan tanggal hari ini (WIB) bila input kosong.
func tanggalDefault(raw string) time.Time {
	t, err := parseTanggalOpsional(raw)
	if err != nil {
		return format.HariIni()
	}
	return t
}

// normalisasiJam mengubah input jam "HH:MM" menjadi "HH:MM:SS".
func normalisasiJam(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) == 5 {
		raw += ":00"
	}
	if _, err := time.Parse("15:04:05", raw); err != nil {
		return "", err
	}
	return raw, nil
}

// pesanError mengirim halaman error sederhana kepada pengguna.
func pesanError(w http.ResponseWriter, code int, judul, detail string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="id"><head><meta charset="utf-8"><title>%s</title>
<link rel="stylesheet" href="/static/style.css"></head>
<body class="halaman-error"><div class="kotak-error"><h1>%d</h1><h2>%s</h2><p>%s</p>
<a class="tombol" href="/">Kembali ke Dashboard</a></div></body></html>`,
		judul, code, judul, detail)
}
