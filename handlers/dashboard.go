package handlers

// Catatan arsitektur (pola yang dipakai di seluruh proyek):
//   - handlers       : hanya menangani HTTP — membaca input, memvalidasi,
//     memanggil internal/store, lalu merender template. Tidak ada SQL di sini.
//   - internal/store : semua akses database (SQL mentah, tanpa ORM).
//   - internal/models: struktur data domain yang disalin dari/ke store.
//   - internal/calc  : aturan bisnis murni yang dapat diuji dengan go test.
//
// Autentikasi: aplikasi memakai sesi berbasis cookie dan tiga peran
// (admin, kasir, member). Pemeriksaan peran ditegakkan di server pada
// middleware wajibPeran, bukan sekadar dengan menyembunyikan menu.

import (
	"net/http"

	"seduhpoin/internal/format"
	"seduhpoin/internal/models"
)

// halamanDashboard adalah titik masuk aplikasi pada path "/".
//
// Dashboard ini berisi ringkasan operasional (omzet, transaksi, promo) yang
// relevan untuk pemilik. Kasir diarahkan ke form transaksi karena itu
// pekerjaan utamanya, member diarahkan ke portalnya sendiri, dan
// pengunjung yang belum login diarahkan ke halaman login.
func (a *App) halamanDashboard(w http.ResponseWriter, r *http.Request) {
	id := identitasDari(r)
	if !id.Masuk {
		redirectKeLogin(w, r)
		return
	}
	switch id.Peran {
	case models.RoleMember:
		http.Redirect(w, r, "/portal", http.StatusSeeOther)
		return
	case models.RoleKasir:
		http.Redirect(w, r, "/transaksi", http.StatusSeeOther)
		return
	}
	a.dashboard(w, r)
}

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
	a.render(w, r, "dashboard/list.html", View{
		Title: "Dashboard",
		Menu:  "dashboard",
		Data:  data,
	})
}
