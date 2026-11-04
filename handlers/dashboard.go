package handlers

// Catatan arsitektur (pola yang dipakai di seluruh proyek):
//   - handlers       : hanya menangani HTTP — membaca input, memvalidasi,
//     memanggil internal/store, lalu merender template. Tidak ada SQL di sini.
//   - internal/store : semua akses database (SQL mentah, tanpa ORM).
//   - internal/models: struktur data domain yang disalin dari/ke store.
//   - internal/calc  : aturan bisnis murni yang dapat diuji dengan go test.
//
// Tidak ada login/sesi: aplikasi memakai satu peran (kasir/admin) sesuai
// spesifikasi tugas akhir, sehingga seluruh menu dapat diakses langsung.

import (
	"net/http"

	"seduhpoin/internal/format"
)

// dashboard menampilkan ringkasan operasional kedai.
//
// Angka yang ditampilkan seluruhnya dihitung ulang dari database saat halaman
// dibuka (bukan disimpan), sehingga selalu mencerminkan data terkini.
func (a *App) dashboard(w http.ResponseWriter, r *http.Request) {
	// Hari ini dalam zona WIB; dipakai sebagai batas bawah transaksi "hari ini".
	hariIni := format.HariIni()
	data, err := a.Store.AmbilDashboard(r.Context(), hariIni)
	if err != nil {
		// Kegagalan database tidak mematikan server; pengguna melihat pesan jelas.
		pesanError(w, http.StatusInternalServerError, "Gagal memuat dashboard", err.Error())
		return
	}
	a.render(w, "dashboard/list.html", View{
		Title: "Dashboard",
		Menu:  "dashboard",
		Data:  data,
	})
}
