package handlers

import (
	"html/template"
	"net/http"
	"strings"

	"seduhpoin/internal/calc"
	"seduhpoin/internal/format"
)

// View adalah data umum yang dikirim ke setiap halaman.
type View struct {
	Title string // judul halaman (header)
	Menu  string // menu sidebar yang sedang aktif
	Error string // pesan error untuk ditampilkan di halaman
	Data  any    // data khusus halaman
}

// templateFuncs berisi helper yang boleh dipakai di dalam template.
var templateFuncs = template.FuncMap{
	"rupiah":            format.Rupiah,
	"angka":             format.Angka,
	"tanggal":           format.Tanggal,
	"tanggalSingkat":    format.TanggalSingkat,
	"bulan":             format.Bulan,
	"labelBulan":        format.LabelBulan,
	"labelBulanPanjang": format.LabelBulanPanjang,
	"keyBulan":          format.KeyBulan,
	"persen":            func(n int) string { return itoa(n) + "%" },
	"lower":             strings.ToLower,
	"upper":             strings.ToUpper,
	"statusBadge":       statusBadge,
	"sisaKuota":         func(kuota, terpakai int) int { return kuota - terpakai },
	"sub":               func(a, b int64) int64 { return a - b },
	"nilaiPoin":         func(p int64) string { return format.Rupiah(p * calc.NilaiTukarPoin) },
	"now":               func() string { return format.Tanggal(format.WIB()) },
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	negatif := n < 0
	if negatif {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if negatif {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// statusBadge memetakan status transaksi menjadi kelas CSS badge.
func statusBadge(status string) string {
	switch status {
	case "SELESAI":
		return "badge badge-ok"
	case "BATAL":
		return "badge badge-batal"
	default:
		return "badge badge-grey"
	}
}

// daftarTemplate berisi file layout + partial yang dipakai semua halaman.
var daftarTemplate = []string{
	"templates/layout.html",
	"templates/header.html",
	"templates/sidebar.html",
	"templates/footer.html",
}

// render mem-parse layout, partial, dan halaman lalu mengeksekusinya.
// Parsing dilakukan per-request supaya perubahan template langsung terlihat
// tanpa perlu restart server (cukup untuk kebutuhan tugas akhir).
func (a *App) render(w http.ResponseWriter, page string, view View) {
	files := append(append([]string{}, daftarTemplate...), "templates/"+page)

	tmpl, err := template.New("base").Funcs(templateFuncs).ParseFiles(files...)
	if err != nil {
		http.Error(w, "Gagal memuat template: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout", view); err != nil {
		http.Error(w, "Gagal merender halaman: "+err.Error(), http.StatusInternalServerError)
	}
}
