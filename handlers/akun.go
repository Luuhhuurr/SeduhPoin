package handlers

import (
	"net/http"
	"strings"

	"seduhpoin/internal/auth"
	"seduhpoin/internal/models"
)

// registerAkun mendaftarkan halaman ganti kata sandi.
//
// Halaman ini terbuka untuk semua peran yang sudah login, karena setiap
// pengguna harus bisa mengganti kata sandinya sendiri.
func (a *App) registerAkun(mux *http.ServeMux) {
	mux.HandleFunc("GET /akun/sandi", a.akses("", a.halamanGantiSandi))
	mux.HandleFunc("POST /akun/sandi", a.akses("", a.prosesGantiSandi))
}

// halamanGantiSandi menampilkan form ganti kata sandi.
//
// Identifier akun TIDAK dibaca dari form: targetnya ditentukan dari sesi
// yang sedang login. Dengan begitu pengguna tidak bisa mengganti sandi
// akun orang lain hanya dengan memanipulasi parameter.
func (a *App) halamanGantiSandi(w http.ResponseWriter, r *http.Request) {
	id := identitasDari(r)

	a.render(w, r, "akun/sandi.html", View{
		Title: "Ganti Kata Sandi",
		Menu:  "akun",
		Error: ambilError(r),
		Data: map[string]any{
			"Identitas": id,
			// "saya" menandai halaman ini dibuka karena login pertama
			// memaksa penggantian sandi awal.
			"Wajib": r.URL.Query().Get("saya") == "1",
		},
	})
}

// prosesGantiSandi memproses form ganti kata sandi.
//
// Alur: users wajib membuktikan tahu sandi lamanya (kecuali admin yang
// menekan tombol reset sandi orang lain), lalu sandi baru harus sesuai
// aturan: minimal 6 karakter dan berbeda dari sandi lama.
func (a *App) prosesGantiSandi(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		pesanError(w, http.StatusBadRequest, "Data form tidak valid", err.Error())
		return
	}

	id := identitasDari(r)
	sandiLama := r.FormValue("sandi_lama")
	sandiBaru := r.FormValue("sandi_baru")
	ulangi := r.FormValue("sandi_ulang")

	tujuan := "/akun/sandi"
	if r.FormValue("saya") == "1" {
		tujuan = "/akun/sandi?saya=1"
	}

	if !auth.PasswordValid(sandiBaru) {
		redirect(w, r, tujuan, "Kata sandi baru minimal "+itoa(auth.PanjangMinimumSandi)+" karakter.")
		return
	}
	if sandiBaru != ulangi {
		redirect(w, r, tujuan, "Konfirmasi kata sandi tidak sama dengan kata sandi baru.")
		return
	}
	if sandiBaru == sandiLama {
		redirect(w, r, tujuan, "Kata sandi baru harus berbeda dari kata sandi lama.")
		return
	}

	switch id.Peran {
	case models.RoleMember:
		// Member harus menyebut sandi lama sebagai verifikasi pemilik akun.
		hash, err := a.Store.HashMember(r.Context(), id.MemberID)
		if err != nil {
			pesanError(w, http.StatusInternalServerError, "Gagal memuat akun", err.Error())
			return
		}
		if !auth.CheckPassword(hash, sandiLama) {
			redirect(w, r, tujuan, "Kata sandi lama salah.")
			return
		}
		hashBaru, err := auth.HashPassword(sandiBaru)
		if err != nil {
			pesanError(w, http.StatusInternalServerError, "Gagal memproses kata sandi", err.Error())
			return
		}
		if err := a.Store.SetPasswordMember(r.Context(), id.MemberID, hashBaru); err != nil {
			pesanError(w, http.StatusInternalServerError, "Gagal menyimpan kata sandi", err.Error())
			return
		}
		// Sesi lama dicabut (kata sandi sudah berbeda), lalu sesi baru
		// dibuat supaya member tetap login dan pesan suksesnya terlihat.
		_ = a.Store.HapusSemuaSesiMember(r.Context(), id.MemberID)
		memberID := id.MemberID
		if err := a.buatSesi(w, r, models.JenisMember, nil, &memberID, models.RoleMember, id.Nama); err != nil {
			pesanError(w, http.StatusInternalServerError, "Gagal membuat sesi baru", err.Error())
			return
		}

	default:
		// Staf (admin dan kasir) wajib menyebut sandi lamanya. Admin yang
		// perlu memasang sandi baru untuk akun orang lain memakai tombol
		// "Atur Ulang Kata Sandi" di halaman detail akun, bukan halaman ini.
		hash, err := a.Store.HashPengguna(r.Context(), id.PenggunaID)
		if err != nil {
			pesanError(w, http.StatusInternalServerError, "Gagal memuat akun", err.Error())
			return
		}
		if !auth.CheckPassword(hash, sandiLama) {
			redirect(w, r, tujuan, "Kata sandi lama salah.")
			return
		}
		hashBaru, err := auth.HashPassword(sandiBaru)
		if err != nil {
			pesanError(w, http.StatusInternalServerError, "Gagal memproses kata sandi", err.Error())
			return
		}
		if err := a.Store.SetPasswordPengguna(r.Context(), id.PenggunaID, hashBaru, false); err != nil {
			pesanError(w, http.StatusInternalServerError, "Gagal menyimpan kata sandi", err.Error())
			return
		}
		// Semua sesi lama dicabut supaya sandi baru langsung berlaku, lalu
		// sesi baru dibuat untuk pengguna ini supaya ia tetap login.
		_ = a.Store.HapusSemuaSesiPengguna(r.Context(), id.PenggunaID)
		penggunaID := id.PenggunaID
		if err := a.buatSesi(w, r, models.JenisPengguna, &penggunaID, nil, id.Peran, id.Nama); err != nil {
			pesanError(w, http.StatusInternalServerError, "Gagal membuat sesi baru", err.Error())
			return
		}
	}

	redirectSukses(w, r, berandaUntuk(id.Peran), "Kata sandi berhasil diganti.")
}

// bersihkanUsername merapikan input username: huruf kecil, tanpa spasi.
func bersihkanUsername(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	return strings.ReplaceAll(raw, " ", "")
}
