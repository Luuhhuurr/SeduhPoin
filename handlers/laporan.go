package handlers

import (
	"net/http"
	"time"

	"seduhpoin/internal/format"
)

// registerLaporan mendaftarkan rute ketiga laporan.
//
// Hanya admin (pemilik) yang boleh melihat laporan; kasir menangani
// transaksi harian, bukan analisis.
func (a *App) registerLaporan(mux *http.ServeMux) {
	mux.HandleFunc("GET /laporan/promo", a.akses(peranAdmin, a.laporanBiayaPromo))
	mux.HandleFunc("GET /laporan/poin", a.akses(peranAdmin, a.laporanPoinBerlaku))
	mux.HandleFunc("GET /laporan/tingkat", a.akses(peranAdmin, a.laporanMemberPerTingkat))
}

// filterLaporan membaca parameter filter periode dan cabang dari query string.
//
// Filter kosong berarti "semua data". Batas atas diperlakukan inklusif:
// tanggal "sampai" dikembalikan sebagai 00:00 hari berikutnya agar transaksi
// pada tanggal tersebut tetap ikut terhitung.
func filterLaporan(r *http.Request) (dari *time.Time, sampai *time.Time, cabangID int64, pesan string) {
	query := r.URL.Query()

	tanggalDari, err := parseTanggalOpsional(query.Get("dari"))
	if err != nil {
		return nil, nil, 0, "Tanggal awal tidak valid. Gunakan format YYYY-MM-DD."
	}
	tanggalSampai, err := parseTanggalOpsional(query.Get("sampai"))
	if err != nil {
		return nil, nil, 0, "Tanggal akhir tidak valid. Gunakan format YYYY-MM-DD."
	}
	if !tanggalDari.IsZero() {
		dari = &tanggalDari
	}
	if !tanggalSampai.IsZero() {
		// Batas atas inklusif: dipakai sampai 00:00 hari berikutnya.
		besok := tanggalSampai.AddDate(0, 0, 1)
		sampai = &besok
	}
	cabangID = parseInt64(query.Get("cabang_id"), 0)
	return dari, sampai, cabangID, ""
}

// laporanBiayaPromo menampilkan laporan biaya promo (FR-18).
//
// Isi laporan: per promo -> jumlah transaksi, total potongan, sisa kuota
// voucher, dan biaya tukar poin pada kolom terpisah.
func (a *App) laporanBiayaPromo(w http.ResponseWriter, r *http.Request) {
	dari, sampai, cabangID, pesan := filterLaporan(r)
	if pesan != "" {
		redirect(w, r, "/laporan/promo", pesan)
		return
	}

	hasil, err := a.Store.LaporanBiayaPromo(r.Context(), dari, sampai, cabangID)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal menyusun laporan biaya promo", err.Error())
		return
	}
	cabang, err := a.Store.DaftarCabang(r.Context(), "")
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal memuat daftar cabang", err.Error())
		return
	}

	a.render(w, r, "laporan/promo.html", View{
		Title: "Laporan Biaya Promo",
		Menu:  "laporan-promo",
		Error: ambilError(r),
		Data: map[string]any{
			"Laporan":  hasil,
			"Cabang":   cabang,
			"CabangID": cabangID,
			"Dari":     r.URL.Query().Get("dari"),
			"Sampai":   r.URL.Query().Get("sampai"),
		},
	})
}

// laporanPoinBerlaku menampilkan laporan poin berlaku (FR-19).
//
// Menampilkan total poin aktif beserta nilai rupiahnya, dikelompokkan per
// bulan kedaluwarsa.
func (a *App) laporanPoinBerlaku(w http.ResponseWriter, r *http.Request) {
	hasil, err := a.Store.LaporanPoinBerlaku(r.Context(), format.HariIni())
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal menyusun laporan poin berlaku", err.Error())
		return
	}

	a.render(w, r, "laporan/poin.html", View{
		Title: "Laporan Poin Berlaku",
		Menu:  "laporan-poin",
		Data: map[string]any{
			"Laporan": hasil,
			"HariIni": format.HariIni(),
		},
	})
}

// laporanMemberPerTingkat menampilkan laporan member per tingkat (FR-20).
//
// Untuk setiap akhir bulan: jumlah member Silver, jumlah member Gold, total
// member, dan jumlah member baru pada bulan tersebut.
func (a *App) laporanMemberPerTingkat(w http.ResponseWriter, r *http.Request) {
	hariIni := format.HariIni()
	// Default: 6 bulan terakhir sampai hari ini.
	dari := hariIni.AddDate(0, -5, 0)
	sampai := hariIni

	query := r.URL.Query()
	if raw := query.Get("dari"); raw != "" {
		t, err := parseTanggal(raw)
		if err != nil {
			redirect(w, r, "/laporan/tingkat", "Tanggal awal tidak valid. Gunakan format YYYY-MM-DD.")
			return
		}
		dari = t
	}
	if raw := query.Get("sampai"); raw != "" {
		t, err := parseTanggal(raw)
		if err != nil {
			redirect(w, r, "/laporan/tingkat", "Tanggal akhir tidak valid. Gunakan format YYYY-MM-DD.")
			return
		}
		sampai = t
	}

	hasil, err := a.Store.LaporanMemberPerTingkat(r.Context(), dari, sampai)
	if err != nil {
		pesanError(w, http.StatusInternalServerError, "Gagal menyusun laporan member per tingkat", err.Error())
		return
	}

	a.render(w, r, "laporan/tingkat.html", View{
		Title: "Laporan Member per Tingkat",
		Menu:  "laporan-tingkat",
		Error: ambilError(r),
		Data: map[string]any{
			"Laporan": hasil,
			"Dari":    dari.Format("2006-01-02"),
			"Sampai":  sampai.Format("2006-01-02"),
		},
	})
}
